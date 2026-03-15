#!/usr/bin/env bash

echo "Stopping and removing test nodes..."
for i in $(seq 1 25); do
    docker stop node-$i 2>/dev/null || true
    docker rm node-$i 2>/dev/null || true
done
echo "Done!"
