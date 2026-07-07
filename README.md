# Pod-Proxier

[![Go Version](https://img.shields.io/github/go-mod/go-version/cylonchau/pod-proxier)](https://golang.org/)
[![License](https://img.shields.io/github/license/cylonchau/pod-proxier)](LICENSE)

**Pod-Proxier** is a dynamic TCP proxy for Kubernetes Pods and Services, powered by HAProxy. It allows developers to create temporary, TTL-managed port mappings to specific Pods or Services for remote debugging (e.g., jprofiler, debug port...), database access, or troubleshooting.

## Features

- **V1: Direct Pod Mapping**: Provides a **direct connection** to specific Pods with TTL-based dynamic mapping. Mapping automatically reverts to the default backend after expiration.
- **V2: Service-based Mapping**: Automatically discovers and proxies Services in allowed namespaces to a defined port range, facilitating access to Service ClusterIPs.
- **HAProxy Powered**: High-performance TCP proxying using HAProxy Data Plane API.
- **TTL Management**: Automated resource cleanup and proxy reset for dynamic mappings.


## Deployment

### Using Helm

Install with both V1 and V2 enabled:

```bash
helm upgrade pod-proxier ./helm \
  --install \
  --namespace infra \
  --create-namespace \
  --set podProxier.enableV1=true \
  --set podProxier.enableV2=true
```

Install V1 only:

```bash
helm upgrade pod-proxier ./helm \
  --install \
  --namespace infra \
  --create-namespace \
  --set podProxier.enableV1=true \
  --set podProxier.enableV2=false \
  --set ports.defaultMapPort=8849 \
  --set ports.jprofilerPortName=jprofiler-port
```

V1 exposes one fixed HAProxy entry port. Traffic flows from `NodeIP:8849` to the selected Pod IP and the Pod container port whose name matches `ports.jprofilerPortName`.

Install V2 only:

```bash
helm upgrade pod-proxier ./helm \
  --install \
  --namespace infra \
  --create-namespace \
  --set podProxier.enableV1=false \
  --set podProxier.enableV2=true \
  --set podProxier.portName=debug \
  --set podProxier.portRangeStart=9000 \
  --set podProxier.portRangeEnd=9100 \
  --set podProxier.allowedNamespaces[0]=default
```

V2 discovers Services in `podProxier.allowedNamespaces` and maps matching Service ports to the configured port range.

Enable a cloud LoadBalancer Service by setting `service.enabled=true`, `service.type=LoadBalancer`, and `service.cloudProvider`.

AWS NLB:

```bash
helm upgrade pod-proxier ./helm \
  --install \
  --namespace infra \
  --create-namespace \
  --set service.enabled=true \
  --set service.type=LoadBalancer \
  --set service.cloudProvider=aws
```

This automatically adds:

```yaml
service.beta.kubernetes.io/aws-load-balancer-type: nlb
```

GCP internal LoadBalancer:

```bash
helm upgrade pod-proxier ./helm \
  --install \
  --namespace infra \
  --create-namespace \
  --set service.enabled=true \
  --set service.type=LoadBalancer \
  --set service.cloudProvider=gcp
```

This automatically adds:

```yaml
cloud.google.com/load-balancer-type: Internal
```

When `service.type=LoadBalancer`, `service.cloudProvider` must be one of `aws` or `gcp`. Extra annotations can still be added with `service.annotations`.

### Build from Source

```bash
docker build --platform linux/amd64 -t pod-proxier:latest .
```

### Run out kubernetes cluster

```bash
./pod-proxier \
    --kubeconfig ~/.kube/config \
    --enable-v1 \
    --enable-v2 \
    --listen-port 3343 \
    --api-addr http://127.0.0.1:5555 \
    --default-map-port 8849 \
    --port-range-start 9000 \
    --port-range-end 9100 \
    --allowed-namespaces default \
    --max-mapping-time 10800
```


## Configuration

### Command Line Flags

| Flag                   | Description                           | Default                 |
| :--------------------- | :------------------------------------ | :---------------------- |
| `--kubeconfig`         | Path to the kubernetes auth config    | `~/.kube/config`        |
| `--enable-v1`          | Enable V1 pod proxy functionality     | `false`                 |
| `--enable-v2`          | Enable V2 service proxy functionality | `false`                 |
| `--listen-port`        | Server internal API port              | `3343`                  |
| `--api-addr`           | HAProxy DataPlaneAPI address          | `http://127.0.0.1:5555` |
| `--default-map-port`   | HAProxy public entry port             | `8849`                  |
| `--port-range-start`   | V2 service mapping start port         | `9000`                  |
| `--port-range-end`     | V2 service mapping end port           | `9100`                  |
| `--allowed-namespaces` | Allowed namespaces for V2             | `default`               |
| `--max-mapping-time`   | Max TTL for V1 mapping (seconds)      | `10800`                 |
| `--check-timeout`      | Health check timeout (seconds)        | `10800`                 |
| `--off-check`          | Disable health check                  | `false`                 |

## Usage & API

### V1: Create a Pod Mapping

**Endpoint**:

```
POST /api/v1/mapping
```

**Request Body**:

```json
{
  "pod_name": "namespace/pod-name",
  "time": 3600
}
```

**Example**:

```bash
curl -XPOST "http://localhost:3343/api/v1/mapping" \
     -H "Content-Type: application/json" \
     -d '{"pod_name": "default/my-pod", "time": 3600}'
```

### V1: Query Pod Existence

**Endpoint**: 

```
GET /api/v1/mapping?pod_name=namespace/pod-name`
```
