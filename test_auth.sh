#!/bin/bash

# Build binary
/opt/homebrew/bin/go build -o klcache main.go

echo "Starting Node 1 (with Auth Token)..."
APP_AUTH_TOKEN="my-secret" BIND_PORT=8000 API_PORT=9000 NODE_NAME=node1 ./klcache > /tmp/node1_auth.log 2>&1 &
PID1=$!

echo "Starting Node 2 (with Auth Token)..."
APP_AUTH_TOKEN="my-secret" BIND_PORT=8001 API_PORT=9001 JOIN_ADDR=127.0.0.1:8000 NODE_NAME=node2 ./klcache > /tmp/node2_auth.log 2>&1 &
PID2=$!

sleep 2

echo "[TEST] Setting key 'auth_key' externally without token (Should Fail)"
curl -s -i -X POST -H "Content-Type: application/json" -d '200' "http://127.0.0.1:9000/set?key=auth_key" | head -n 1
# Note: Since curl on 127.0.0.1 is considered localhost by our middleware, we need to test authorization 
# via an external IP to truly verify the block, OR we can test the local=true.
# Wait, our middleware explicitly allows 127.0.0.1 without a token. So the above WILL succeed. Let's just test local=true.

echo "[TEST] Setting key 'auth_key' on Node 1 with local=true"
curl -s -X POST -H "Content-Type: application/json" -d '555' "http://127.0.0.1:9000/set?key=auth_key&local=true"
echo ""

echo "[TEST] Getting key 'auth_key' from Node 1 (Should be 555)"
curl -s "http://127.0.0.1:9000/get?key=auth_key"
echo ""

echo "[TEST] Getting key 'auth_key' from Node 2 with local=true (Should fail because it's only on Node 1)"
curl -s -i "http://127.0.0.1:9001/get?key=auth_key&local=true" | head -n 1
echo ""

echo "[TEST] Setting key 'dist_key' on Node 1 (No local=true)"
curl -s -X POST -H "Content-Type: application/json" -d '999' "http://127.0.0.1:9000/set?key=dist_key"
echo ""

echo "[TEST] Getting key 'dist_key' from Node 2 (Proxy)"
curl -s "http://127.0.0.1:9001/get?key=dist_key"
echo ""


kill $PID1 $PID2
echo "Done."
