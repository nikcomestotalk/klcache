#!/bin/bash

# Build binary
/opt/homebrew/bin/go build -o klcache main.go

echo "--- Spinning up Cluster A (USER-SERVICE) ---"
APP_NAME="user-service" BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache > /tmp/node1_clusterA.log 2>&1 &
PID1=$!

APP_NAME="user-service" BIND_PORT=8001 API_PORT=9001 NODE_NAME=node2 ./klcache > /tmp/node2_clusterA.log 2>&1 &
PID2=$!

echo "--- Spinning up Cluster B (AUTH-SERVICE) ---"
APP_NAME="auth-service" BIND_PORT=8002 API_PORT=9002 NODE_NAME=node3 ./klcache > /tmp/node3_clusterB.log 2>&1 &
PID3=$!

APP_NAME="auth-service" BIND_PORT=8003 API_PORT=9003 NODE_NAME=node4 ./klcache > /tmp/node4_clusterB.log 2>&1 &
PID4=$!

echo "Waiting 6 seconds for mDNS discovery to complete for both discrete clusters..."
sleep 6

echo "[TEST] Setting key 'cross_test' on Node 1 (Cluster A)"
curl -s -X POST -H "Content-Type: application/json" -d '123' "http://127.0.0.1:9000/set?key=cross_test"
echo ""

echo "[TEST] Getting key 'cross_test' from Node 2 (Cluster A - Proxy should succeed)"
curl -s "http://127.0.0.1:9001/get?key=cross_test"
echo ""

echo "[TEST] Getting key 'cross_test' from Node 4 (Cluster B - Should be Not Found because they are isolated)"
curl -s -i "http://127.0.0.1:9003/get?key=cross_test" | head -n 1
echo ""

echo "[TEST] Killing processes"
kill $PID1 $PID2 $PID3 $PID4

echo "Integration Test Finished. Checking node logs for mDNS channels:"
echo "--- NODE 1 (Cluster A) ---"
tail -n 12 /tmp/node1_clusterA.log
echo "--- NODE 3 (Cluster B) ---"
tail -n 12 /tmp/node3_clusterB.log
