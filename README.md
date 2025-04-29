## Pod proxier

Pod 代理，提供一个单Pod代理的入口，通过公共地址可以访问一个Pod，并且可以切换哪个Pod可以访问，用于单Pod调试

## Quick start

```bash
cd helm && helm upgrade pod-proxier -n infra --install .
```

## API

Create a pod map

```bash
curl -XPOST "host:3433/api/v1/mapping?pod_name={namespace}/{pod_name}&time=3600"
```