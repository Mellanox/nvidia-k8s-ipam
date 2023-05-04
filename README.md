# nvidia-k8s-ipam
NVIDIA IPAM plugin

## Usage

### Deploy IPAM

_NOTE: This command will deploy latest dev build_

```
kubectl apply -f https://raw.githubusercontent.com/Mellanox/nvidia-k8s-ipam/main/deploy/nv-ipam.yaml
```

### Create config
```
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
        "pool1": {"subnet": "192.168.0.0/16", "perNodeBlockSize": 100 , "gateway": "192.168.0.1"},
        "pool2": {"subnet": "172.16.0.0/16", "perNodeBlockSize": 50 , "gateway": "172.16.0.1"}
      },
      "nodeSelector": {"kubernetes.io/os": "linux"}
    }
EOF

```

## CNI config

Example config for bridge CNI

```
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
