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
2. node daemon ([ipam-node](#ipam-node))
3. IPAM CNI plugin ([nv-ipam](#nv-ipam))

### ipam-controller

A Kubernetes(K8s) controller that Watches on IPPools CRs in a predefined Namespace.
It then proceeds by assiging each node via IPPools Status a cluster unique range of IPs of the defined IP Pools.

#### Validation webhook

ipam-controller implements validation webhook for IPPool resource.
The webhook can prevent the creation of IPPool resources with invalid configurations. 
Supported X.509 certificate management system should be available in the cluster to enable the webhook.
Currently supported systems are [certmanager](https://cert-manager.io/) and
[Openshift certificate management](https://docs.openshift.com/container-platform/4.13/security/certificates/service-serving-certificate.html)

Activation of the validation webhook is optional. Check the [Deployment](#deployment) section for details.

### ipam-node

The daemon is responsible for:
- perform initial setup and installation of nv-ipam CNI plugin
- perform allocations of the IPs and persist them on the disk
- run periodic jobs, such as cleanup of the stale IP address allocations

A node daemon provides GRPC service, which nv-ipam CNI plugin uses to request IP address allocation/deallocation.
IPs are allocated from the provided IP Block assigned by ipam-controller for the node.
To determine the cluster unique IP Block for the defined IP Pool, ipam-node watches K8s API
for the IPPool objects and extracts IP Block information from IPPool Status.

### nv-ipam

An IPAM CNI plugin that handles CNI requests according to the CNI spec.
To allocate/deallocate IP address nv-ipam calls GRPC API of ipam-node daemon.

### IP allocation flow

1. User (cluster administrator) defines a set of named IP Pools to be used for IP allocation
of container interfaces via IPPool CRD (more information in [Configuration](#configuration) section)

_Example_:

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  subnet: 192.168.0.0/16
  perNodeBlockSize: 24
  gateway: 192.168.0.1
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
        - key: kubernetes.io/role
          operator: In
          values:
            - worker
```

2. ipam-controller calculates and assigns unique IP Blocks for each Node via IPPool Status:

_Example_:

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  gateway: 192.168.0.1
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
      - key: kubernetes.io/role
        operator: In
        values:
        - worker
  perNodeBlockSize: 24
  subnet: 192.168.0.0/16
status:
  allocations:
  - endIP: 192.168.0.24
    nodeName: host-a
    startIP: 192.168.0.1
  - endIP: 192.168.0.48
    nodeName: host-b
    startIP: 192.168.0.25
  - endIP: 192.168.0.72
    nodeName: host-c
    startIP: 192.168.0.49
  - endIP: 192.168.0.96
    nodeName: k8s-master
    startIP: 192.168.0.73

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

ipam-controller accepts configuration using command line flags and IPPools CRs.

#### Flags

```text
Logging flags:

      --log-flush-frequency duration                                                                                                                                                                  
                Maximum number of seconds between log flushes (default 5s)
      --log-json-info-buffer-size quantity                                                                                                                                                            
                [Alpha] In JSON format with split output streams, the info messages can be buffered for a while to increase performance. The default value of zero bytes disables buffering. The size can
                be specified as number of bytes (512), multiples of 1000 (1K), multiples of 1024 (2Ki), or powers of those (3M, 4G, 5Mi, 6Gi). Enable the LoggingAlphaOptions feature gate to use this.
      --log-json-split-stream                                                                                                                                                                         
                [Alpha] In JSON format, write error messages to stderr and info messages to stdout. The default is to write a single stream to stdout. Enable the LoggingAlphaOptions feature gate to use
                this.
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

      --health-probe-bind-address string                                                                                                                                                              
                The address the probe endpoint binds to. (default ":8081")
      --ippools-namespace string                                                                                                                                                                      
                The name of the namespace to watch for IPPools CRs (default "kube-system")
      --kubeconfig string                                                                                                                                                                             
                Paths to a kubeconfig. Only required if out-of-cluster.
      --leader-elect                                                                                                                                                                                  
                Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
      --leader-elect-namespace string                                                                                                                                                                 
                Determines the namespace in which the leader election resource will be created. (default "kube-system")
      --metrics-bind-address string                                                                                                                                                                   
                The address the metric endpoint binds to. (default ":8080")
      --webhook                                                                                                                                                                                       
                Enable validating webhook server as a part of the controller

```

#### IPPool CR

ipam-controller accepts IP Pools configuration via IPPool CRs.
Multiple IPPool CRs can be created, with different NodeSelectors.

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: my-pool
  namespace: kube-system
spec:
  subnet: 192.168.0.0/16
  perNodeBlockSize: 100
  gateway: 192.168.0.1
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
        - key: kubernetes.io/role
          operator: In
          values:
            - worker
```

* `spec`: contains the IP pool configuration
  * `subnet`: IP Subnet of the pool
  * `gateway`: Gateway IP of the subnet
  * `perNodeBlockSize`: the number of IPs of IP Blocks allocated to Nodes.
  * `nodeSelector`: A list of node selector terms. The terms are ORed. Each term can have a list of matchExpressions that are ANDed. Only the nodes that match the provided labels will get assigned IP Blocks for the defined pool.

> __Notes:__
>
> * pool name is composed of alphanumeric letters separated by dots(`.`) underscores(`_`) or hyphens(`-`)
> * `perNodeBlockSize` minimum size is 2
> * `subnet` must be large enough to accommodate at least one `perNodeBlockSize` block of IPs

### ipam-node configuration

ipam-node accepts configuration via command line flags.
Options that begins with the `cni-` prefix are used to create the config file for nv-ipam CNI.
All other options are for the ipam-node daemon itself.

```text
Logging flags:

      --log-flush-frequency duration                                                                                                                                                                 
                Maximum number of seconds between log flushes (default 5s)
      --log-json-info-buffer-size quantity                                                                                                                                                           
                [Alpha] In JSON format with split output streams, the info messages can be buffered for a while to increase performance. The default value of zero bytes disables buffering. The size
                can be specified as number of bytes (512), multiples of 1000 (1K), multiples of 1024 (2Ki), or powers of those (3M, 4G, 5Mi, 6Gi). Enable the LoggingAlphaOptions feature gate to use this.
      --log-json-split-stream                                                                                                                                                                        
                [Alpha] In JSON format, write error messages to stderr and info messages to stdout. The default is to write a single stream to stdout. Enable the LoggingAlphaOptions feature gate to
                use this.
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

Node daemon flags:

      --bind-address string                                                                                                                                                                          
                GPRC server bind address. e.g.: tcp://127.0.0.1:9092, unix:///var/lib/foo (default "unix:///var/lib/cni/nv-ipam/daemon.sock")
      --health-probe-bind-address string                                                                                                                                                             
                The address the probe endpoint binds to. (default ":8081")
      --kubeconfig string                                                                                                                                                                            
                Paths to a kubeconfig. Only required if out-of-cluster.
      --metrics-bind-address string                                                                                                                                                                  
                The address the metric endpoint binds to. (default ":8080")
      --node-name string                                                                                                                                                                             
                The name of the Node on which the daemon runs
      --store-file string                                                                                                                                                                            
                Path of the file which used to store allocations (default "/var/lib/cni/nv-ipam/store")
      --ippools-namespace string
                The name of the namespace to watch for IPPools CRs. (default "kube-system")

Shim CNI Configuration flags:

      --cni-bin-dir string                                                                                                                                                                           
                CNI binary directory (default "/opt/cni/bin")
      --cni-conf-dir string                                                                                                                                                                          
                shim CNI config: path with config file (default "/etc/cni/net.d/nv-ipam.d")
      --cni-daemon-call-timeout int                                                                                                                                                                  
                shim CNI config: timeout for IPAM daemon calls (default 5)
      --cni-daemon-socket string                                                                                                                                                                     
                shim CNI config: IPAM daemon socket path (default "unix:///var/lib/cni/nv-ipam/daemon.sock")
      --cni-log-file string                                                                                                                                                                          
                shim CNI config: path to log file for shim CNI (default "/var/log/nv-ipam-cni.log")
      --cni-log-level string                                                                                                                                                                         
                shim CNI config: log level for shim CNI (default "info")
      --cni-nv-ipam-bin-file string                                                                                                                                                                  
                nv-ipam binary file path (default "/nv-ipam")
      --cni-skip-nv-ipam-binary-copy                                                                                                                                                                 
                skip nv-ipam binary file copy
      --cni-skip-nv-ipam-config-creation                                                                                                                                                             
                skip config file creation for nv-ipam CNI

```

### nv-ipam CNI configuration

nv-ipam accepts the following CNI configuration:

```json
{
    "type": "nv-ipam",
    "poolName": "my-pool",
    "daemonSocket": "unix:///var/lib/cni/nv-ipam/daemon.sock",
    "daemonCallTimeoutSeconds": 5,
    "confDir": "/etc/cni/net.d/nv-ipam.d",
    "logFile": "/var/log/nv-ipam-cni.log",
    "logLevel": "info"
}
```

* `type` (string, required): CNI plugin name, MUST be `"nv-ipam"`
* `poolName` (string, optional): name of the IP Pool to be used for IP allocation.
It is possible to allocate two IPs for the interface from different pools by specifying pool names separated by coma,
e.g. `"my-ipv4-pool,my-ipv6-pool"`. The primary intent to support multiple pools is a dual-stack use-case when an 
interface should have two IP addresses: one IPv4 and one IPv6. (default: network name as provided in CNI call)
* `daemonSocket` (string, optional): address of GRPC server socket served by IPAM daemon
* `daemonCallTimeoutSeconds` (integer, optional): timeout for GRPC calls to IPAM daemon
* `confDir` (string, optional): path to configuration dir. (default: `"/etc/cni/net.d/nv-ipam.d"`)
* `logFile` (string, optional): log file path. (default: `"/var/log/nv-ipam-cni.log"`)
* `logLevel` (string, optional): logging level. one of: `["verbose", "debug", "info", "warning", "error", "panic"]`.  (default: `"info"`)

## Deployment

### Deploy IPAM plugin

> _NOTE:_ These commands will deploy latest dev build with default configuration

The plugin can be deployed with kustomize.

Supported overlays are:

`no-webhook` - deploy without webhook

```shell
kubectl kustomize https://github.com/mellanox/nvidia-k8s-ipam/deploy/overlays/no-webhook?ref=main | kubectl apply -f -
```

`certmanager` - deploy with webhook to the Kubernetes cluster where certmanager is available

```shell
kubectl kustomize https://github.com/mellanox/nvidia-k8s-ipam/deploy/overlays/certmanager?ref=main | kubectl apply -f -
```

`openshift` - deploy with webhook to the Openshift cluster

```shell
kubectl kustomize https://github.com/mellanox/nvidia-k8s-ipam/deploy/overlays/openshift?ref=main | kubectl apply -f -
```

### Create IPPool CR

```shell
cat <<EOF | kubectl apply -f -
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: my-pool
  namespace: kube-system
spec:
  subnet: 192.168.0.0/16
  perNodeBlockSize: 100
  gateway: 192.168.0.1
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
        - key: kubernetes.io/role
          operator: In
          values:
            - worker
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
kubectl get ippools.nv-ipam.nvidia.com -A -o jsonpath='{range .items[*]}{.metadata.name}{"\n"} {range .status.allocations[*]}{"\t"}{.nodeName} => Start IP: {.startIP} End IP: {.endIP}{"\n"}{end}{"\n"}{end}'

pool1
 	host-a => Start IP: 192.168.0.1 End IP: 192.168.0.24
	host-b => Start IP: 192.168.0.25 End IP: 192.168.0.48
	host-c => Start IP: 192.168.0.49 End IP: 192.168.0.72
	k8s-master => Start IP: 192.168.0.73 End IP: 192.168.0.96

pool2
 	host-a => Start IP: 172.16.0.1 End IP: 172.16.0.50
	host-b => Start IP: 172.16.0.51 End IP: 172.16.0.100
	host-c => Start IP: 172.16.0.101 End IP: 172.16.0.150
	k8s-master => Start IP: 172.16.0.151 End IP: 172.16.0.200
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

* Before removing a node from cluster, drain all workloads to ensure proper cleanup of IPs on node.
* IP Block allocated to a node with Gateway IP in its range will have one less IP than what defined in perNodeBlockSize, deployers should take this into account.
* Defining multiple IP Pools while supported, was not thoroughly testing

## Contributing

We welcome your feedback and contributions to this project. Please see the [CONTRIBUTING.md](https://github.com/Mellanox/nvidia-k8s-ipam/blob/main/CONTRIBUTING.md)
for contribution guidelines.
