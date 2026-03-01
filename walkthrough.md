# KLCache Walkthrough

Welcome to `KLCache`, a decentralized, peer-to-peer Key-Value cache sidecar application built in Go.

## Overview
This application is designed to act as a sidecar to your primary services. It forms a decentralized cluster using HashiCorp's **Gossip Protocol** (`memberlist`) and distributes keys across this cluster using a **Consistent Hashing Ring** (`buraksezer/consistent`). 

This architecture guarantees:
- **No Central Server**: Every node is equal; no master daemon is required.
- **Dynamic Rebalancing**: As sidecars join or leave the cluster, the Hash Ring recalculates key ownership dynamically.
- **Transparent API**: Your local application simply talks to its local sidecar (`http://127.0.0.1:9000`). If the key belongs to another node, the sidecar seamlessly proxies the HTTP request and returns the result.

## Architecture & Data Flow
1. **Zero-Config Node Initialization**: A sidecar is launched with a unique `NODE_NAME`, `BIND_PORT` (for Gossip), and `API_PORT` (for HTTP requests). There's no need to point to a central or static element!
2. **mDNS Auto-Discovery & Segregation**: The node broadcasts its presence via Multicast DNS on `_klcache_<APP_CLUSTER_ID>._tcp` using HashiCorp's `mdns`. This means that in orchestration layers like Kubernetes, if you define `APP_CLUSTER_ID="user-service"`, the sidecars will exclusively cluster with other pods running the exact same application. Sidecars running alongside a different app (e.g. `APP_CLUSTER_ID="auth-service"`) are completely segregated.
3. **Gossip Protocol Formation**: Upon discovering a peer's IP and gossip port, the node connects to the cluster intelligently using HashiCorp's `memberlist` library, sharing metadata via custom `memberlist.Delegate` to share API ports.
4. **Consistent Hashing**: All nodes individually run a Hash Ring populated with the cluster members. Since the hashing calculation (`xxhash`) is deterministic, all nodes agree on who owns which key based on the node names.
5. **Data Operations & Security**: 
   - A middleware layer ensures the API only responds to requests originating from `127.0.0.1` (the main local application), verifying tight-coupling.
   - When a `SET`/`GET` operates on the local API, the proxy layer hashes the `key`.
   - If the key maps to the current node, the operation impacts the thread-safe `cache.Store` locally.
   - If the key maps to a remote node, the node forwards the request directly to the owner, appending the `Authorization` header containing the system's `APP_AUTH_TOKEN`.
   - All requests support appending `?local=true`. This completely bypasses the cluster algorithms, and forces the cache to read/write locally.

## Verification
### 1. Unit Testing
We successfully wrote and executed unit tests:
- `cache/store_test.go`: Verified type assertions (allowing only string keys mapped to string, bool, int, float) and concurrency read/writes using >100 Goroutines to validate `sync.RWMutex`.
- `cluster/hashring_test.go`: Validated the integrity of the `xxhash` key mappings when nodes are inserted or removed.

### 2. Integration Output
We spun up 3 instances locally:
`Node 1 (Port 9000)`, `Node 2 (Port 9001)`, `Node 3 (Port 9002)`

We sent a key specifically to Node 1:
```bash
curl -s -X POST -H "Content-Type: application/json" -d '100' "http://127.0.0.1:9000/set?key=foo"
```
The HashRing mapped the string `"foo"` to Node 2. We observed Node 1 correctly identifying this and proxying the command to `http://127.0.0.1:9001/set?key=foo`. 

Later pulling `/get?key=foo` from Node 3 resulted in Node 3 correctly identifying Node 2 as the owner and proxying the request successfully. 
All test logs returned `OK` and `{"key":"foo","value":100}`!

## How to Run it Yourself
To deploy locally, compile the binary and launch multiple tabs. Nodes will discover each other automatically via Multicast DNS:

```bash
# Tab 1: First node
BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache

# Tab 2: Second node (No JOIN_ADDR needed!)
BIND_PORT=8001 API_PORT=9001 NODE_NAME=node2 ./klcache

# Tab 3: Third node (No JOIN_ADDR needed!)
BIND_PORT=8002 API_PORT=9002 NODE_NAME=node3 ./klcache
```

Now use standard HTTP calls to query any of the endpoints, and the proxy layer leverages the consistent hash ring deterministically!
