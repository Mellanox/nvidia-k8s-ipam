# Extra CNI args in network config

## Supported arguments

The NV-IPAM plugin supports the following [args in network config](https://www.cni.dev/docs/conventions/#args-in-network-config): 

| Argument               | Type                     | Description                                                                                                                                                                                              |
|------------------------|--------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| ips                    | `[]string`               | Request static IPs from the pool                                                                                                                                                                         |
| poolNames              | `[]string,` max len is 2 | Name of the pools to be used for IP allocation.  _The field has higher priority than `ipam.poolName`                                                                                                    |
| poolType               | `string`                 | Type (`ippool`, `cidrpool`) of the pool which is referred by the `poolNames`. _The field has higher priority than_ `ipam.poolType`                                                                       |
| allocateDefaultGateway | `bool`                   | Request to allocate pool's default gateway as interface IP address for the container. Pool must have the gateway when this argument is used. The argument can't be used together with static IP request. |



## Use CNI args in Multus annotation

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: static-ip-pod
  annotations:
    k8s.v1.cni.cncf.io/networks: '
      [{"name": "mynet",
        "cni-args":
            {"poolNames": ["pool1"],
             "poolType": "cidrpool",
             "allocateDefaultGateway": true}}]'
spec:
  containers:
  - name: samplepod
    command: ["/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait"]
    image: ubuntu
  terminationGracePeriodSeconds: 1
```
