#!/bin/bash

# Build binary
/opt/homebrew/bin/go build -o klcache main.go

# Start Node 1 (Seed)
BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache > /tmp/node1.log 2>&1 &
PID1=$!

# Wait for seed to initialize
sleep 2

# Start Node 2 (Joins Node 1)
BIND_PORT=8001 API_PORT=9001 NODE_NAME=node2 JOIN_ADDR=127.0.0.1:8000 ./klcache > /tmp/node2.log 2>&1 &
PID2=$!

# Start Node 3 (Joins Node 1)
BIND_PORT=8002 API_PORT=9002 NODE_NAME=node3 JOIN_ADDR=127.0.0.1:8000 ./klcache > /tmp/node3.log 2>&1 &
PID3=$!

# Wait for cluster to stabilize
sleep 3

# Send data to Node 1
echo "[TEST] Setting key 'foo' with value '100' via Node 1"
curl -s -X POST -H "Content-Type: application/json" -d '100' "http://127.0.0.1:9000/set?key=foo"
echo ""

# Get data from Node 2
echo "[TEST] Getting key 'foo' via Node 2"
curl -s "http://127.0.0.1:9001/get?key=foo"
echo ""

# Get data from Node 3
echo "[TEST] Getting key 'foo' via Node 3"
curl -s "http://127.0.0.1:9002/get?key=foo"
echo ""

# Which node owns 'foo'? Let's check the logs
echo "[TEST] Killing processes"
kill $PID1 $PID2 $PID3

echo "Integration Test Finished. Outputting node logs for inspection:"
echo "--- NODE 1 ---"
tail -n 10 /tmp/node1.log
echo "--- NODE 2 ---"
tail -n 10 /tmp/node2.log
echo "--- NODE 3 ---"
tail -n 10 /tmp/node3.log
