#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

IMAGE_NAME="fleettui-test-node"

declare -A NODE_ROLES
NODE_ROLES=(
    [1]="offline-no-port"
    [2]="offline-no-port"
    [3]="offline-no-port"
    [4]="offline-broken"
    [5]="offline-broken"
    [6]="high-cpu"
    [7]="high-cpu"
    [8]="high-cpu"
    [9]="high-cpu"
    [10]="high-mem"
    [11]="high-mem"
    [12]="high-mem"
    [13]="high-mem"
    [14]="network-server"
    [15]="network-client"
    [16]="network-client"
    [17]="failed-systemd"
    [18]="failed-systemd"
    [19]="normal"
    [20]="normal"
    [21]="normal"
    [22]="normal"
    [23]="normal"
    [24]="normal"
    [25]="normal"
)

OFFLINE_NODES=()
REACHABLE_NODES=()

echo "Building test node image..."
docker build -t $IMAGE_NAME .

echo "Creating docker network..."
docker network create fleettui-net 2>/dev/null || true

echo ""
echo "Spawning 25 test nodes..."

for i in $(seq 1 25); do
    role="${NODE_ROLES[$i]}"
    
    if [ "$role" = "offline-no-port" ]; then
        docker run -d \
            --name "node-$i" \
            --hostname "node-$i" \
            --network "fleettui-net" \
            -e "NODE_ROLE=$role" \
            $IMAGE_NAME
        OFFLINE_NODES+=("node-$i")
        echo "Created node-$i (offline-no-port) - NO PORT MAPPED"
    else
        port=$((2200 + i))
        docker run -d \
            --name "node-$i" \
            --hostname "node-$i" \
            --network "fleettui-net" \
            -p $port:22 \
            -e "NODE_ROLE=$role" \
            --cpus="0.5" \
            --memory="256m" \
            $IMAGE_NAME
        REACHABLE_NODES+=("node-$i:$port")
        echo "Created node-$i on port $port (role: $role)"
    fi
done

echo ""
echo "========================================"
echo "All 25 nodes created!"
echo "========================================"
echo ""
echo "Role distribution:"
echo "  - offline-no-port: 3 nodes (1-3)"
echo "  - offline-broken: 2 nodes (4-5)"
echo "  - high-cpu: 4 nodes (6-9)"
echo "  - high-mem: 4 nodes (10-13)"
echo "  - network: 3 nodes (14-16) - iperf3 to gateway"
echo "  - failed-systemd: 2 nodes (17-18)"
echo "  - normal: 7 nodes (19-25)"
echo ""
echo "Reachable nodes SSH examples:"
for node in "${REACHABLE_NODES[@]}"; do
    name="${node%:*}"
    port="${node#*:}"
    echo "  ssh root@127.0.0.1 -p $port  # $name"
done
echo ""
echo "Offline nodes (no port mapped - will show as unreachable):"
for node in "${OFFLINE_NODES[@]}"; do
    echo "  $node"
done
echo ""
echo "Run './stop-nodes.sh' to stop all nodes"
