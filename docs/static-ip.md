# Static IP address configuration

The nv-ipam plugin supports three options to request a static IP address: 
* [IP field in CNI_ARGS](https://www.cni.dev/docs/conventions/#cni_args) - supports only on IP address
* ["ips" key in "args" in network config](https://www.cni.dev/docs/conventions/#args-in-network-config)
* ["ips" Capability](https://www.cni.dev/docs/conventions/#well-known-capabilities)


# Configure static IP with multus annotaion

## Create IPPool CR

```yaml
apiVersion: nv-ipam.nvidia.com/v1alpha1
kind: IPPool
metadata:
  name: pool1
  namespace: kube-system
spec:
  subnet: "192.168.0.0/16"
  perNodeBlockSize: 128
  exclusions: 
  - startIP: "192.168.0.200" # exclude single IP
    endIP: "192.168.0.200"
  gateway: "192.168.0.1"
```

## Create NetworkAttachmentDefinition

```yaml
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: mynet
spec:
  config: '{
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "macvlan",
          "master": "eth0",
          "mode": "bridge",
          "capabilities": {"ips": true},
          "ipam": {
            "type": "nv-ipam",
            "poolName": "pool1"
          }
        }
      ]
    }'

```

> _NOTE:_ Multus will use the CNI_ARGS configuration method, which doesn't support multiple IP addresses if `"capabilities": {"ips": true}` is not set.

## Create test pod with Multus annotaion

### Option 1 - use `ips` field

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: static-ip-pod
  annotations:
    k8s.v1.cni.cncf.io/networks: '
      [{"name": "mynet",
        "ips": ["192.168.0.200"]}]'
spec:
  containers:
  - name: samplepod
    command: ["/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait"]
    image: ubuntu
  terminationGracePeriodSeconds: 1

```

### Option 2 - use `cni-args["ips"]` field

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: static-ip-pod
  annotations:
    k8s.v1.cni.cncf.io/networks: '
      [{"name": "mynet",
        "cni-args":
            {"ips": ["192.168.0.200"]}}]'
spec:
  containers:
  - name: samplepod
    command: ["/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait"]
    image: ubuntu
  terminationGracePeriodSeconds: 1
```

## Implementation details

* NV-IPAM stores information about assigned IPs on each node separately. This means that the plugin can prevent the assigning of the same static IP multiple times on the same node, but it cannot guarantee that the static IP address will only be assigned once across the cluster.

* NV-IPAM can statically allocate only IPs that belong to the pool's subnet.

    * for IP pools it is possible to statically allocate any IP from `IPPool.spec.subnet` network on any node which match the pool.

    * For CIDR pools, it is possible to statically allocate only IPs that match the allocated node's `CIDRPool.status.allocations[<node's allocation index>].prefix`. This means that on node-a, it is possible to allocate IPs from the node-a prefix, but it is not possible to allocate IPs from the node-b prefix. It is recommended that `nodeSelector` be used for pods that need to use static IPs with CIDR pools.

* NV-IPAM can statically allocate pool's gateway IP.

* NV-IPAM can statically allocate IPs that are excluded by `CIDRPool.spec.exclusions` and `IPPool.spec.exclusions`.
