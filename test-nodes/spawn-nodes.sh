#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

IMAGE_NAME="fleettui-test-node"
CONTAINER_COUNT=25
BASE_PORT=2200

echo "Building test node image..."
docker build -t $IMAGE_NAME .

echo "Spawning $CONTAINER_COUNT test nodes..."
for i in $(seq 1 $CONTAINER_COUNT); do
    port=$((BASE_PORT + i))
    docker run -d \
        --name node-$i \
        --hostname node-$i \
        -p $port:22 \
        $IMAGE_NAME
    echo "Created node-$i on port $port"
done

echo ""
echo "All $CONTAINER_COUNT nodes created!"
echo "SSH example: ssh root@127.0.0.1 -p 2201"
