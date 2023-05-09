# NVIDIA IPAM Plugin

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mellanox/nvidia-k8s-ipam)](https://goreportcard.com/report/github.com/Mellanox/nvidia-k8s-ipam)
[![Coverage Status](https://coveralls.io/repos/github/Mellanox/nvidia-k8s-ipam/badge.svg)](https://coveralls.io/github/Mellanox/nvidia-k8s-ipam)
[![Build, Test, Lint](https://github.com/Mellanox/nvidia-k8s-ipam/actions/workflows/build-test-lint.yml/badge.svg?event=push)](https://github.com/Mellanox/nvidia-k8s-ipam/actions/workflows/build-test-lint.yml)
[![CodeQL](https://github.com/Mellanox/nvidia-k8s-ipam/actions/workflows/codeql.yml/badge.svg)](https://github.com/Mellanox/nvidia-k8s-ipam/actions/workflows/codeql.yml)
[![Image push](https://github.com/Mellanox/nvidia-k8s-ipam/actions/workflows/image-push-main.yml/badge.svg?event=push)](https://github.com/Mellanox/nvidia-k8s-ipam/actions/workflows/image-push-main.yml)

An IP Address Management (IPAM) CNI plugin designed to operate in a Kubernetes environment.
This Plugins allows to assign IP addresses dynamically across the cluster while keeping speed
and performance in mind.

IP subnets are defined by the user as named _IP Pools_, then for each IP Pool a unique _IP Block_ is assigned
to each K8s Node which is used to assign IPs to container network interfaces.

NVIDIA IPAM plugin currently support only IP allocation for K8s Secondary Networks.
e.g Additional networks provided by [multus CNI plugin](https://github.com/k8snetworkplumbingwg/multus-cni).

This repository is in its first steps of development, APIs may change in a non backward compatible manner.

## Introduction

NVIDIA IPAM plugin consists of 3 main components:

1. controller ([ipam-controller](#ipam-controller))
2. node agent ([ipam-node](#ipam-node))
3. IPAM CNI plugin ([nv-ipam](#nv-ipam))

### ipam-controller

A Kubernetes(K8s) controller that Watches on a predefined K8s ConfigMap for defined IP Pools.
It then proceeds in reconciling K8s Node objects by assiging each node via `ipam.nvidia.com/ip-blocks`
annotation a cluster unique range of IPs of the defined IP Pools.

### ipam-node

A node agent that performs initial setup and installation of nv-ipam CNI plugin.

### nv-ipam

An IPAM CNI plugin that allocates IPs for a given interface out of the defined IP Pool as provided
via CNI configuration.

IPs are allocated out of the provided IP Block assigned by ipam-controller for the node.
To determine the cluster unique IP Block for the defined IP Pool, nv-ipam CNI queries K8s API
for the Node object and extracts IP Block information from node annotation.

nv-ipam plugin currently leverages [host-local](https://www.cni.dev/plugins/current/ipam/host-local/)
IPAM to allocate IPs from the given range.

### IP allocation flow

1. User (cluster administrator) defines a set of named IP Pools to be used for IP allocation
of container interfaces via Kubernetes ConfigMap (more information in [Configuration](#configuration) section)

_Example_:

```json
{
    "pools":  {
        "my-pool": {"subnet": "192.188.0.0/16", "perNodeBlockSize": 24, "gateway": "192.168.0.1"}
    },
    "nodeSelector": {
      "kubernetes.io/os": "linux"
    }
}
```

2. ipam-controller calculates and assigns unique IP Blocks for each Node via annotation

_Example_:

```yaml
annotations:
    ipam.nvidia.com/ip-blocks: '{
    "my-pool": {"startIP": "192.168.0.2", "endIP": "192.168.0.25", "gateway": "192.168.0.1", "subnet": "192.168.0.0/16"}
    }'
```

3. User specifies nv-ipam as IPAM plugin in CNI configuration

_Example macvlan CNI configuration_:

```json
{
  "type": "macvlan",
  "cniVersion": "0.3.1",
  "master": "enp3s0f0np0",
  "mode": "bridge",
  "ipam": {
      "type": "nv-ipam",
      "poolName": "my-pool"
    }
}
```

4. nv-ipam plugin, as a result of CNI ADD command allocates a free IP from the IP Block of the
corresponding IP Pool that was allocated for the node

## Configuration

### ipam-controller configuration

ipam-controller accepts configuration using command line flags and K8s configMap

#### Flags

```text
Logging flags:

      --log-flush-frequency duration                                                                                                                                                           
                Maximum number of seconds between log flushes (default 5s)
      --log-json-info-buffer-size quantity                                                                                                                                                     
                [Alpha] In JSON format with split output streams, the info messages can be buffered for a while to increase performance. The default value of zero bytes disables buffering. The
                size can be specified as number of bytes (512), multiples of 1000 (1K), multiples of 1024 (2Ki), or powers of those (3M, 4G, 5Mi, 6Gi). Enable the LoggingAlphaOptions feature
                gate to use this.
      --log-json-split-stream                                                                                                                                                                  
                [Alpha] In JSON format, write error messages to stderr and info messages to stdout. The default is to write a single stream to stdout. Enable the LoggingAlphaOptions feature gate
                to use this.
      --logging-format string                                                                                                                                                                  
                Sets the log format. Permitted formats: "json" (gated by LoggingBetaOptions), "text". (default "text")
  -v, --v Level                                                                                                                                                                                
                number for the log level verbosity
      --vmodule pattern=N,...                                                                                                                                                                  
                comma-separated list of pattern=N settings for file-filtered logging (only works for text log format)

Common flags:

      --feature-gates mapStringBool                                                                                                                                                            
                A set of key=value pairs that describe feature gates for alpha/experimental features. Options are:
                AllAlpha=true|false (ALPHA - default=false)
                AllBeta=true|false (BETA - default=false)
                ContextualLogging=true|false (ALPHA - default=false)
                LoggingAlphaOptions=true|false (ALPHA - default=false)
                LoggingBetaOptions=true|false (BETA - default=true)
      --version                                                                                                                                                                                
                print binary version and exit

Controller flags:

      --config-name string                                                                                                                                                                     
                The name of the ConfigMap which holds controller configuration (default "nvidia-k8s-ipam-config")
      --config-namespace string                                                                                                                                                                
                The name of the namespace where ConfigMap with controller configuration exist (default "kube-system")
      --health-probe-bind-address string                                                                                                                                                       
                The address the probe endpoint binds to. (default ":8081")
      --kubeconfig string                                                                                                                                                                      
                Paths to a kubeconfig. Only required if out-of-cluster.
      --leader-elect                                                                                                                                                                           
                Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
      --leader-elect-namespace string                                                                                                                                                          
                Determines the namespace in which the leader election resource will be created. (default "kube-system")
      --metrics-bind-address string                                                                                                                                                            
                The address the metric endpoint binds to. (default ":8080")
```

#### ConfigMap

ipam-controller accepts IP Pool configuration via configMap with a pre-defined key named `config`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nvidia-k8s-ipam-config
  namespace: kube-system
data:
  config: |
    {
      "pools": {
        "my-pool": {"subnet": "192.168.0.0/16", "perNodeBlockSize": 100 , "gateway": "192.168.0.1"}
      },
      "nodeSelector": {"kubernetes.io/os": "linux"}
    }
```

* `pools`: contains a set of named IP Pools keyed by name
  * `subnet`: IP Subnet of the pool
  * `gateway`: Gateway IP of the subnet
  * `perNodeBlockSize`: the number of IPs of IP Blocks allocated to Nodes.
* `nodeSelector`: a map<string, string> of node selector labels, only nodes that match the provided labels will get assigned IP Blocks for the defined pools

> __Notes:__
>
> * pool name is composed of alphanumeric letters separated by dots(`.`) undersocres(`_`) or hyphens(`-`)
> * `perNodeBlockSize` minimum size is 2
> * `subnet` must be large enough to accommodate at least one `perNodeBlockSize` block of IPs

### ipam-node configuration

ipam-node accepts configuration via command line flags

```text
      --cni-bin-dir string                    CNI binary directory (default "/host/opt/cni/bin")
      --cni-conf-dir string                   CNI config directory (default "/host/etc/cni/net.d")
  -h, --help                                  show help message and quit
      --host-local-bin-file string            host-local binary file path (default "/host-local")
      --nv-ipam-bin-file string               nv-ipam binary file path (default "/nv-ipam")
      --nv-ipam-cni-data-dir string           nv-ipam CNI data directory (default "/host/var/lib/cni/nv-ipam")
      --nv-ipam-cni-data-dir-host string      nv-ipam CNI data directory on host (default "/var/lib/cni/nv-ipam")
      --nv-ipam-kubeconfig-file-host string   kubeconfig for nv-ipam (default "/etc/cni/net.d/nv-ipam.d/nv-ipam.kubeconfig")
      --nv-ipam-log-file string               nv-ipam log file (default "/var/log/nv-ipam-cni.log")
      --nv-ipam-log-level string              nv-ipam log level (default "info")
      --skip-host-local-binary-copy           skip host-loca binary file copy
      --skip-nv-ipam-binary-copy              skip nv-ipam binary file copy
```

### nv-ipam CNI configuration

nv-ipam accepts the following CNI configuration:

```json
{
    "type": "nv-ipam",
    "poolName": "my-pool",
    "kubeconfig": "/etc/cni/net.d/nv-ipam.d/nv-ipam.kubeconfig",
    "dataDir": "/var/lib/cni/nv-ipam",
    "confDir": "/etc/cni/net.d/nv-ipam.d",
    "logFile": "/var/log/nv-ipam-cni.log",
    "logLevel": "info"
}
```

* `type` (string, required): CNI plugin name, MUST be `"nv-ipam"`
* `poolName` (string, optional): name of the IP Pool to be used for IP allocation. (default: network name as provided in CNI call)
* `kubeconfig` (string, optional): path to kubeconfig file. (default: `"/etc/cni/net.d/nv-ipam.d/nv-ipam.kubeconfig"`)
* `dataDir` (string, optional): path to data dir. (default: `/var/lib/cni/nv-ipam`)
* `confDir` (string, optional): path to configuration dir. (default: `"/etc/cni/net.d/nv-ipam.d"`)
* `logFile` (string, optional): log file path. (default: `"/var/log/nv-ipam-cni.log"`)
* `logLevel` (string, optional): logging level. one of: `["verbose", "debug", "info", "warning", "error", "panic"]`.  (default: `"info"`)

## Deployment

### Deploy IPAM plugin

> _NOTE:_ This command will deploy latest dev build with default configuration

```shell
kubectl apply -f https://raw.githubusercontent.com/Mellanox/nvidia-k8s-ipam/main/deploy/nv-ipam.yaml
```

### Create ipam-controller config

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: nvidia-k8s-ipam-config
  namespace: kube-system
data:
  config: |
    {
      "pools": {
        "my-pool": {"subnet": "192.168.0.0/16", "perNodeBlockSize": 100 , "gateway": "192.168.0.1"}
      },
      "nodeSelector": {"kubernetes.io/os": "linux"}
    }
EOF
```

### Create CNI configuration

_Example config for bridge CNI:_

```shell
cat > /etc/cni/net.d/10-mynet.conf <<EOF
{
    "cniVersion": "0.4.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "mytestbr",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "nv-ipam",
        "poolName": "pool1"
    }
}
EOF
```

## Debugging

View allocated IP Blocks:

```shell
kubectl get nodes -o=custom-columns='NAME:metadata.name,ANNOTATION:metadata.annotations.ipam\.nvidia\.com/ip-blocks'
```

View network status of pods:

```shell
kubectl get pods -o=custom-columns='NAME:metadata.name,NODE:spec.nodeName,NETWORK-STATUS:metadata.annotations.k8s\.v1\.cni\.cncf\.io/network-status'
```

View ipam-controller logs:

```shell
kubectl -n kube-system logs <nv-ipam-controller pod name>
```

View ipam-node logs:

```shell
kubectl -n kube-system logs <nv-ipam-node-ds pod name>
```

View nv-ipam CNI logs:

```shell
cat /var/log/nv-ipam-cni.log
```

> _Note:_ It is recommended to increase log level to `"debug"` when debugging CNI issues.
> This can be done by either editing CNI configuration (e.g editing NetworkAttachmentDefinition)
> or locally on a node by editing nv-ipam config file (default: `/etc/init.d/nv-ipam.d/nv-ipam.conf`

## Limitations

* Deleting an IP Pool from config map while there are pods scheduled on nodes with IPs from deleted Pool, deleting these pods will fail (CNI CMD DEL fails)
* Before removing a node from cluster, drain all workloads to ensure proper cleanup of IPs on node.
* IP Block allocated to a node with Gateway IP in its range will have one less IP than what defined in perNodeBlockSize, deployers should take this into account.
* IPv6 not supported
* Allocating Multiple IPs per interface is not supported
* Defining multiple IP Pools while supported, was not thoroughly testing

## Contributing

We welcome your feedback and contributions to this project. Please see the [CONTRIBUTING.md](https://github.com/Mellanox/nvidia-k8s-ipam/blob/main/CONTRIBUTING.md)
for contribution guidelines.
