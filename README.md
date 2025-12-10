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

NVIDIA IPAM plugin supports allocation of IP ranges and Network prefixes for nodes.

* [IPPool CR](#ippool-cr) can be used to create an IP Pool. This type of pool can be used to split a single IP network into multiple unique IP ranges and allocate them for nodes. The nodes will use the same network mask as the original IP network.

  This pool type is useful for flat networks where Pods from all nodes have L2 connectivity with each other.

  **Example:**
	```
  network: 192.168.0.0/16
  gateway: 192.168.0.1
  perNodeBlockSize: 4 (amount of IPs)

  node1 will allocate IPs from the following range: 192.168.0.1-192.168.0.4 (gateway is part of the range)

  node2 will allocate IPs from the following range: 192.168.0.5-192.168.0.8

	First Pod on the node1 will get the following IP config:
		IP: 192.168.0.2/16 (gateway IP was skipped)
		Gateway: 192.168.0.1

	First Pod on the node2 will get the following IP config:
		IP: 192.168.0.5/16
		Gateway: 192.168.0.1

  Pods from different nodes can have L2 connectivity.
  ```


* [CIDRPool CR](#cidrpool-cr) can be used to create a CIDR Pool. This type of pool can be used to split a large network into multiple unique smaller subnets and allocate them for nodes. Each node will have a specific gateway and network mask with a node's subnet size.

   This pool type is useful for routed networks where Pods on each node should use unique subnets and node-specific gateways to communicate with Pods from other nodes.

  **Example:**
	```
  cidr: 192.168.0.0/16
  perNodeNetworkPrefix: 24 (subnet size)
  gatewayIndex: 1

  node1 will allocate IPs from the following subnet: 192.168.0.0/24

  node2 will allocate IPs from the following subnet: 192.168.1.0/24

	First Pod on the node1 will get the following IP config:
		IP: 192.168.0.2/24
		Gateway: 192.168.0.1

	First Pod on the node2 will get the following IP config:
		IP: 192.168.1.2/24
		Gateway: 192.168.1.1

  Pods from different nodes don't have L2 connectivity, routing is required.
  ```


NVIDIA IPAM plugin currently support only IP allocation for K8s Secondary Networks.
e.g Additional networks provided by [multus CNI plugin](https://github.com/k8snetworkplumbingwg/multus-cni).

This repository is in its first steps of development, APIs may change in a non backward compatible manner.

## Introduction

NVIDIA IPAM plugin consists of 3 main components:

1. controller ([ipam-controller](#ipam-controller))
2. node daemon ([ipam-node](#ipam-node))
3. IPAM CNI plugin ([nv-ipam](#nv-ipam))

### ipam-controller

A Kubernetes(K8s) controller that Watches on IPPools and CIDRPools CRs in a predefined Namespace.
It then proceeds by assiging each node via CR's Status a cluster unique range of IPs or subnet that is used by the ipam-node. 

#### Validation webhook

ipam-controller implements validation webhook for IPPool and CIDRPool resources.
The webhook can prevent the creation of resources with invalid configurations. 
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

IPs are allocated from the provided IP Blocks and prefixes assigned by ipam-controller for the node.
ipam-node watches K8s API
for the IPPool and CIDRPool objects and extracts allocated ranges or prefixes from the status field.

### nv-ipam

An IPAM CNI plugin that handles CNI requests according to the CNI spec.
To allocate/deallocate IP address nv-ipam calls GRPC API of ipam-node daemon.

### IP allocation flow

#### IPPool

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
        - key: node-role.kubernetes.io/worker
          operator: Exists
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
      - key: node-role.kubernetes.io/worker
        operator: Exists
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
      "poolName": "pool1"
    }
}
```

4. nv-ipam plugin, as a result of CNI ADD command allocates a free IP from the IP Block of the
corresponding IP Pool that was allocated for the node

#### CIDRPool

1. User (cluster administrator) defines a set of named CIDR Pools to be used for prefix allocation
of container interfaces via CIDRPool CRD (more information in [Configuration](#configuration) section)

_Example_:

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: CIDRPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  cidr: 192.168.0.0/16
  gatewayIndex: 1
  perNodeNetworkPrefix: 24
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
      - key: node-role.kubernetes.io/worker
        operator: Exists

```

2. ipam-controller calculates and assigns unique subnets (with `perNodeNetworkPrefix` size) for each Node via CIDRPool Status:

_Example_:

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: CIDRPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  cidr: 192.168.0.0/16
  gatewayIndex: 1
  perNodeNetworkPrefix: 24
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
      - key: node-role.kubernetes.io/worker
        operator: Exists
status:
  allocations:
  - gateway: 192.168.0.1
    nodeName: host-a
    prefix: 192.168.0.0/24
  - gateway: 192.168.1.1
    nodeName: host-b
    prefix: 192.168.1.0/24

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
      "poolName": "pool1",
      "poolType": "cidrpool"
    }
}
```

4. nv-ipam plugin, as a result of CNI ADD command allocates a free IP from the prefix that was allocated for the node

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

##### IPv4 example

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  subnet: 192.168.0.0/16
  perNodeBlockSize: 100
  gateway: 192.168.0.1
  exclusions: # optional
  - startIP: 192.168.0.10
    endIP: 192.168.0.20
  perNodeExclusions: # optional
  - startIndex: 0
    endIndex: 10
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
        - key: node-role.kubernetes.io/worker
          operator: Exists
  defaultGateway: true # optional
  routes: # optional
  - dst: 5.5.0.0/24
```

##### IPv6 example

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: pool2
  namespace: kube-system
spec:
  subnet: fd52:2eb5:44::/48
  perNodeBlockSize: 500
  gateway: fd52:2eb5:44::1
  nodeSelector:
    nodeSelectorTerms:
    - matchExpressions:
        - key: node-role.kubernetes.io/worker
          operator: Exists
```

##### Fields

* `spec`: contains the IP pool configuration.
  * `subnet`: IP Subnet of the pool.
  * `gateway` (optional): Gateway IP of the subnet.
  * `perNodeBlockSize`: the number of IPs of IP Blocks allocated to Nodes.
  * `exclusions` (optional, list): contains reserved IP addresses that should not be allocated by nv-ipam node component.

    * `startIP`: start IP of the exclude range (inclusive).
    * `endIP`: end IP of the exclude range (inclusive).

  * `perNodeExclusions` (optional, list): contains reserved IP address indexes that should not be allocated by nv-ipam node component. The IP address indexes are relative to the per-node IP block allocated to each node, counting from the start of the allocated range. For example, if a node is allocated the IP block 192.168.0.1-192.168.0.100, then index 0 corresponds to 192.168.0.1, index 1 to 192.168.0.2, and so on. Note: For IPPool, indexes count from the range start; for CIDRPool, indexes count from the subnet start (network address).

    * `startIndex`: start index of the exclude range (inclusive). Must be non-negative.
    * `endIndex`: end index of the exclude range (inclusive). Must be greater than or equal to startIndex and within the perNodeBlockSize range.

  * `nodeSelector` (optional): A list of node selector terms. The terms are ORed. Each term can have a list of matchExpressions that are ANDed. Only the nodes that match the provided labels will get assigned IP Blocks for the defined pool.
  * `defaultGateway` (optional): Add the pool gateway as default gateway in the pod static routes.
  * `routes` (optional, list): contains CIDR to be added in the pod static routes via the pool gateway.

    * `dst`: The destination of the static route, in CIDR notation.

> __Notes:__
>
> * pool name is composed of alphanumeric letters separated by dots(`.`) underscores(`_`) or hyphens(`-`).
> * `perNodeBlockSize` minimum size is 1.
> * `subnet` must be large enough to accommodate at least one `perNodeBlockSize` block of IPs.



#### CIDRPool CR

ipam-controller accepts CIDR Pools configuration via CIDRPool CRs.
Multiple CIDRPool CRs can be created, with different NodeSelectors.

##### IPv4 example

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: CIDRPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  cidr: 192.168.0.0/16
  gatewayIndex: 1
  perNodeNetworkPrefix: 24
  exclusions: # optional
    - startIP: 192.168.0.10
      endIP: 192.168.0.20
  perNodeExclusions: # optional
    - startIndex: 0
      endIndex: 10
  staticAllocations:
    - nodeName: node-33
      prefix: 192.168.33.0/24
      gateway: 192.168.33.10
    - prefix: 192.168.1.0/24
  nodeSelector: # optional
    nodeSelectorTerms:
      - matchExpressions:
          - key: node-role.kubernetes.io/worker
            operator: Exists
  defaultGateway: true # optional
  routes: # optional
  - dst: 5.5.0.0/24
```

##### IPv6 example

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: CIDRPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  cidr: fd52:2eb5:44::/48
  gatewayIndex: 1
  perNodeNetworkPrefix: 120
  nodeSelector: # optional
    nodeSelectorTerms:
      - matchExpressions:
          - key: node-role.kubernetes.io/worker
            operator: Exists

```

##### Point to point prefixes

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: CIDRPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  cidr: 192.168.100.0/24
  perNodeNetworkPrefix: 31
```

##### Fields

* `spec`: contains the CIDR pool configuration.
  * `cidr`: pool's CIDR block which will be split to smaller prefixes(size is define in perNodePrefixSize) and distributed between matching nodes.
  * `gatewayIndex` (optional): `not set` - no gateway, if set automatically use IP with this index from the host prefix as a gateway.
  * `perNodeNetworkPrefix`:  size of the network prefix for each host, the network defined in `cidr` field will be split to multiple networks with this size.
  * `exclusions` (optional, list): contains reserved IP addresses that should not be allocated by nv-ipam node component.

      * `startIP`: start IP of the exclude range (inclusive).
      * `endIP`:  end IP of the exclude range (inclusive).

  * `perNodeExclusions` (optional, list): contains reserved IP address indexes that should not be allocated by nv-ipam node component. The IP address indexes are relative to the per-node prefix allocated to each node, counting from the subnet start (network address). For example, if a node is allocated the prefix 192.168.0.0/24, then index 0 corresponds to 192.168.0.0 (network address), index 1 to 192.168.0.1 (which would be the gateway if gatewayIndex is 1), index 2 to 192.168.0.2, and so on. This indexing is consistent with gatewayIndex.

      * `startIndex`: start index of the exclude range (inclusive). Must be non-negative.
      * `endIndex`: end index of the exclude range (inclusive). Must be greater than or equal to startIndex and within the subnet size defined by perNodeNetworkPrefix.

  * `staticAllocations` (optional, list): static allocations for the pool.

      * `nodeName` (optional): name of the node for static allocation, can be empty in case if the prefix should be preallocated without assigning it for a specific node.
      * `gateway` (optional): gateway for the node, if not set the gateway will be computed from `gatewayIndex`.
      * `prefix`: statically allocated prefix.

  * `nodeSelector`(optional): A list of node selector terms. The terms are ORed. Each term can have a list of matchExpressions that are ANDed. Only the nodes that match the provided labels will get assigned IP Blocks for the defined pool.
  * `defaultGateway` (optional): Add the pool gateway as default gateway in the pod static routes.
  * `routes` (optional, list): contains CIDR to be added in the pod static routes via the pool gateway.

    * `dst`: The destination of the static route, in CIDR notation.

> __Notes:__
>
> * pool name is composed of alphanumeric letters separated by dots(`.`) underscores(`_`) or hyphens(`-`).
> * `perNodeNetworkPrefix` must be equal or smaller (more network bits) then the size of pool's `cidr`.


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
      --cni-force-pool-name
                shim CNI config: force specifying pool name in CNI configuration
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
    "forcePoolName" : false,
    "poolName": "pool1,pool2",
    "poolType": "ippool",
    "daemonSocket": "unix:///var/lib/cni/nv-ipam/daemon.sock",
    "daemonCallTimeoutSeconds": 5,
    "confDir": "/etc/cni/net.d/nv-ipam.d",
    "logFile": "/var/log/nv-ipam-cni.log",
    "logLevel": "info"
}
```

* `type` (string, required): CNI plugin name, MUST be `"nv-ipam"`
* `forcePoolName` (bool, optional): force specifying pool name in CNI configuration.
* `poolName` (string, optional): name of the Pool to be used for IP allocation.
It is possible to allocate two IPs for the interface from different pools by specifying pool names separated by coma,
e.g. `"my-ipv4-pool,my-ipv6-pool"`. The primary intent to support multiple pools is a dual-stack use-case when an 
interface should have two IP addresses: one IPv4 and one IPv6. (default: network name as provided in CNI call)
* `poolType` (string, optional): type (ippool, cidrpool) of the pool which is referred by the `poolName`. The field is case-insensitive. (default: `"ippool"`)
* `daemonSocket` (string, optional): address of GRPC server socket served by IPAM daemon
* `daemonCallTimeoutSeconds` (integer, optional): timeout for GRPC calls to IPAM daemon
* `confDir` (string, optional): path to configuration dir. (default: `"/etc/cni/net.d/nv-ipam.d"`)
* `logFile` (string, optional): log file path. (default: `"/var/log/nv-ipam-cni.log"`)
* `logLevel` (string, optional): logging level. one of: `["verbose", "debug", "info", "warning", "error", "panic"]`.  (default: `"info"`)


### Advanced configuration

* [Static IP address configuration](docs/static-ip.md)

* [Extra CNI args in network config](docs/extra-cni-args.md)


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

## Debugging

### View allocated IP Blocks for a node from IPPool:

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

### View allocated IP Prefixes for a node from CIDRPool:

```shell
kubectl get cidrpools.nv-ipam.nvidia.com -A -o jsonpath='{range .items[*]}{.metadata.name}{"\n"} {range .status.allocations[*]}{"\t"}{.nodeName} => Prefix: {.prefix} Gateway: {.gateway}{"\n"}{end}{"\n"}{end}'

pool1
	host-a => Prefix: 10.0.0.0/24 Gateway: 10.0.0.1
	host-b => Prefix: 10.0.1.0/24 Gateway: 10.0.2.1

```

### View network status of pods:

```shell
kubectl get pods -o=custom-columns='NAME:metadata.name,NODE:spec.nodeName,NETWORK-STATUS:metadata.annotations.k8s\.v1\.cni\.cncf\.io/network-status'
```

### View ipam-controller logs:

```shell
kubectl -n kube-system logs <nv-ipam-controller pod name>
```

### View ipam-node logs:

```shell
kubectl -n kube-system logs <nv-ipam-node-ds pod name>
```

### View nv-ipam CNI logs:

```shell
cat /var/log/nv-ipam-cni.log
```

> _Note:_ It is recommended to increase log level to `"debug"` when debugging CNI issues.
> This can be done by either editing CNI configuration (e.g editing NetworkAttachmentDefinition)
> or locally on a node by editing nv-ipam config file (default: `/etc/init.d/nv-ipam.d/nv-ipam.conf`

## Limitations

* Before removing a node from cluster, drain all workloads to ensure proper cleanup of IPs on node.
* IP Block allocated to a node with Gateway IP in its range will have one less IP than what defined in perNodeBlockSize, deployers should take this into account.

## Contributing

We welcome your feedback and contributions to this project. Please see the [CONTRIBUTING.md](https://github.com/Mellanox/nvidia-k8s-ipam/blob/main/CONTRIBUTING.md)
for contribution guidelines.
