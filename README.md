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

KLCache is a decentralized, peer-to-peer Key-Value cache sidecar application built in Go. It's designed to run alongside your primary services, providing a distributed, scalable caching layer without the need for a central master node or complex configuration.

## Key Features

- **Zero-Config Auto-Discovery & Segregation**: Nodes automatically find each other on the local network using HashiCorp's **mDNS** (`hashicorp/mdns`). By configuring an `APP_CLUSTER_ID`, sidecars dynamically segregate themselves so a cluster solely binds horizontally across identical application pods (e.g. your `auth-service` cache won't collide with your `user-service` cache).
- **Decentralized Cluster Management**: Utilizes the **Gossip Protocol** (`hashicorp/memberlist`) for node health-checking, failure detection, and cluster state dissemination. 
- **Consistent Data Distribution**: Employs **Consistent Hashing** (`buraksezer/consistent` + `cespare/xxhash`) to deterministically route keys to specific nodes. If nodes join or leave, data ownership minimizes re-shuffling.
- **Transparent Request Proxying**: Every node exposes an HTTP API (`/set`, `/get`, `/delete`). If a request targets a key that belongs to a remote node, the current node intercepts it and seamlessly proxies the request to the correct owner. Your app only ever has to talk to `localhost:9000`.
- **Local-Only Overrides**: For operations that must stay on the current machine, add `?local=true` to any API endpoint to bypass clustering entirely.
- **Strict Security Coupling**: By default, the API exclusively accepts traffic from `localhost`. If you add the `APP_AUTH_TOKEN` environment variable, non-localhost traffic (like proxy routing) must pass the Bearer token.
- **Type-Safe Storage**: The in-memory cache strictly validates storing mapped Types (String keys to Strings, Integers, Booleans, or Floats).

## Quick Start

### Building
```bash
go build -o klcache main.go
```

### Running Locally with Auto-Discovery

You can run multiple instances locally. Thanks to mDNS, they will find each other automatically without any initial `JOIN_ADDR` configuration.

**Terminal 1 (Node 1):**
```bash
BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache
```

**Terminal 2 (Node 2):**
```bash
BIND_PORT=8001 API_PORT=9001 NODE_NAME=node2 ./klcache
```

**Terminal 3 (Node 3):**
```bash
BIND_PORT=8002 API_PORT=9002 NODE_NAME=node3 ./klcache
```

You'll quickly see the nodes discovering each other via mDNS logs and forming a cluster!

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
| `BIND_ADDR` | IP Address for Gossip Protocol to bind to | `127.0.0.1` |
| `BIND_PORT` | Port for Gossip Protocol and mDNS | `8000` |
| `API_PORT` | Port for HTTP API and transparent inter-node proxying | `9000` |
| `NODE_NAME` | Unique identifier for the instance | `node-{API_PORT}` |
| `JOIN_ADDR` | Explicit IP:Port of an existing node (Useful if mDNS is disabled/cloud) | `""` |
| `APP_AUTH_TOKEN` | Bearer token to authorize external proxy traffic | `""` |
| `APP_CLUSTER_ID` | Identifies the namespace grouping for mDNS discovery | `"default"` |

## Production Use
For environments where mDNS/multicast is disabled (like AWS VPCs or Kubernetes), you can provide a static IP to `JOIN_ADDR` for the initial seed, or integrate a DNS-based discovery record that resolves to the instances.
