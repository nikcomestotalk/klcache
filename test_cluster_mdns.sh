#!/bin/bash

# Build binary
/opt/homebrew/bin/go build -o klcache main.go

echo "Starting Node 1..."
BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache > /tmp/node1_mdns.log 2>&1 &
PID1=$!

sleep 2

echo "Starting Node 2 (No JOIN_ADDR provided)..."
BIND_PORT=8001 API_PORT=9001 NODE_NAME=node2 ./klcache > /tmp/node2_mdns.log 2>&1 &
PID2=$!

sleep 2

echo "Starting Node 3 (No JOIN_ADDR provided)..."
BIND_PORT=8002 API_PORT=9002 NODE_NAME=node3 ./klcache > /tmp/node3_mdns.log 2>&1 &
PID3=$!

# Wait for mDNS to discover and memberlist to gossip
echo "Waiting 6 seconds for mDNS discovery to complete..."
sleep 6

# Send data to Node 1
echo "[TEST] Setting key 'mdns_test' with value '404' via Node 1"
curl -s -X POST -H "Content-Type: application/json" -d '404' "http://127.0.0.1:9000/set?key=mdns_test"
echo ""

# Get data from Node 3
echo "[TEST] Getting key 'mdns_test' via Node 3 (Proxy test)"
curl -s "http://127.0.0.1:9002/get?key=mdns_test"
echo ""

echo "[TEST] Killing processes"
kill $PID1 $PID2 $PID3

echo "Integration Test Finished. Outputting node logs for inspection:"
echo "--- NODE 1 ---"
tail -n 12 /tmp/node1_mdns.log
echo "--- NODE 2 ---"
tail -n 12 /tmp/node2_mdns.log
echo "--- NODE 3 ---"
tail -n 12 /tmp/node3_mdns.log
