# KLCache
## what is KLCache and why
Sidecar Distributed Cache is a clear, service-scoped caching layer designed to run alongside every application instance. 
Instead of relying on local in-memory caches, which can lead to fragmentation across replicas, or on centralized systems like Redis, which create dependencies between services and shared infrastructure, this project forms a lightweight cluster through service sidecars. 
This setup provides a unified, isolated cache for each service. 
For instance, if the user-service scales to 100 instances, 100 cache sidecars will create a dedicated distributed cache cluster just for that service. This means no shared cache, no noisy neighbors, and no cross-service key collisions. 
## Core Philosophy Service-level isolation: 
Each service owns its cache cluster. 
### Horizontal scalability: 
Cache capacity scales 1:1 with application replicas.
### Decoupled architecture: 
No shared Redis, no global cache tier. 
### Sidecar-native: 
Built for containerized environments like Kubernetes. 
### Opinionated by design: 
Minimal configuration, clear defaults, and predictable behavior. 
#### Why Not Local Memory? 
Local in-memory caches: Break consistency across replicas. Create uneven cache hit rates. Waste memory per pod. 
#### Why Not Shared Redis? 
Centralized caches: Introduce network hops and latency. Couple unrelated services. Create noisy neighbor problems. Become operational bottlenecks. 

What This Solves Unified cache across service replicas. Independent scaling per service. Reduced external infrastructure dependency. Improved performance consistency. Better fault isolation.


# KLCache

KLCache is a **sidecar-first** distributed cache. Run one KLCache container alongside each instance of your application; sidecars for the same application automatically discover each other and form a dedicated cache cluster. No Redis, no shared infrastructure—each service owns its cache.

## Key Features

- **Same-App Discovery**: Sidecars with the same `APP_NAME` form one cluster. Your `user-service` sidecars only talk to other `user-service` sidecars—never to `auth-service` or other apps.
- **Kubernetes-Native Discovery**: In Kubernetes, sidecars discover peers via DNS (headless service). No mDNS or multicast required.
- **Local Dev (mDNS)**: Outside Kubernetes, sidecars use mDNS to find each other on the local network.
- **Decentralized Cluster**: Gossip protocol (`memberlist`) for health-checking and membership; consistent hashing for key distribution.
- **Transparent Proxying**: Your app talks only to `localhost:9000`. If a key lives on another sidecar, the local one proxies the request.
- **Local-Only Override**: Add `?local=true` to any endpoint to bypass clustering.
- **Security**: API accepts localhost by default; set `APP_AUTH_TOKEN` for Bearer auth on inter-pod proxy traffic.
- **Type-Safe Storage**: Keys map to string, int, bool, or float64.

## Quick Start

### Building
```bash
go build -o klcache main.go
```

### Running Locally (mDNS Discovery)

Run multiple sidecar instances on the same machine. They discover each other via mDNS when they share the same `APP_NAME`.

**Terminal 1:**
```bash
APP_NAME=my-app BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache
```

**Terminal 2:**
```bash
APP_NAME=my-app BIND_PORT=8001 API_PORT=9001 NODE_NAME=node2 ./klcache
```

**Terminal 3:**
```bash
APP_NAME=my-app BIND_PORT=8002 API_PORT=9002 NODE_NAME=node3 ./klcache
```

All three form one cluster for `my-app`. A different `APP_NAME` would form a separate cluster.

### Usage (API)
Communicate with your local node (e.g., `http://127.0.0.1:9000`). If the key is hashed to be stored on Node 3, Node 1 will proxy it automatically!

**Set a Value:**
```bash
curl -X POST -H "Content-Type: application/json" -d 'true' "http://127.0.0.1:9000/set?key=is_active"
```

**Get a Value:**
```bash
curl "http://127.0.0.1:9000/get?key=is_active"
# Output: {"key":"is_active","value":true}
```

**Delete a Value:**
```bash
curl -X POST "http://127.0.0.1:9000/delete?key=is_active"
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_NAME` | Application name — sidecars with same name form one cluster | `"default"` (or `APP_CLUSTER_ID`) |
| `BIND_ADDR` | IP for gossip (use `0.0.0.0` in K8s) | `127.0.0.1` local; `0.0.0.0` in K8s |
| `BIND_PORT` | Port for memberlist gossip | `8000` |
| `API_PORT` | Port for HTTP API | `9000` |
| `NODE_NAME` | Unique instance ID (in K8s: pod name via `HOSTNAME`) | `node-{API_PORT}` |
| `POD_IP` | Pod IP in K8s (for advertising reachable address) | — set via downward API |
| `KUBE_NAMESPACE` | Kubernetes namespace (for DNS discovery) | `"default"` or `POD_NAMESPACE` |
| `JOIN_ADDR` | Explicit node to join (overrides discovery) | `""` |
| `APP_AUTH_TOKEN` | Bearer token for inter-pod proxy traffic | `""` |

## Kubernetes Deployment (Sidecar)

Add KLCache as a sidecar container. Use a **headless service** so sidecars can discover each other via DNS.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: user-service
spec:
  clusterIP: None   # Headless — DNS returns all pod IPs
  selector:
    app: user-service
  ports:
    - port: 8000
      name: gossip
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      containers:
        - name: app
          image: your-app:latest
          # Your app talks to localhost:9000
        - name: klcache
          image: klcache:latest
          env:
            - name: APP_NAME
              value: "user-service"   # Must match Service name
            - name: BIND_PORT
              value: "8000"
            - name: API_PORT
              value: "9000"
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: KUBE_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          ports:
            - containerPort: 8000
              name: gossip
            - containerPort: 9000
              name: api
```

Sidecars discover peers by resolving `user-service.{namespace}.svc.cluster.local` (returns all pod IPs). Each sidecar joins the others on the gossip port and forms a cluster.
