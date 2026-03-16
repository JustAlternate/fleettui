#!/usr/bin/env bash
set -e

ROLE="${NODE_ROLE:-normal}"

setup_systemctl_wrapper() {
    cat > /usr/local/bin/systemctl << 'WRAPPER'
#!/bin/bash
case "$*" in
    --failed*)
        echo "test-fail-1.service   loaded failed failed"
        echo "test-fail-2.service   loaded failed failed" 
        exit 0
        ;;
    list-units*|--type=service*)
        echo "ssh.service     loaded active running OpenBSD Secure Shell server"
        echo "test-fail-1.service   loaded failed failed"
        echo "test-fail-2.service   loaded failed failed"
        echo "systemd-journald.service loaded active running Journal Service"
        exit 0
        ;;
    *)
        /usr/bin/systemctl.real "$@"
        ;;
esac
WRAPPER
    chmod +x /usr/local/bin/systemctl
    cp /usr/bin/systemctl /usr/bin/systemctl.real 2>/dev/null || true
}

case "$ROLE" in
    "high-cpu")
        echo "Starting high CPU stress node..."
        stress-ng --cpu 2 --cpu-load 80 --cpu-method seq --timeout 0 &
        ;;

    "high-mem")
        echo "Starting high memory stress node..."
        stress-ng --vm 2 --vm-bytes 60% --timeout 0 &
        ;;

    "network-server")
        echo "Starting iperf3 server..."
        iperf3 -s -D
        ;;

    "network-client")
        echo "Starting iperf3 client (blasting to node-14)..."
        while true; do
            iperf3 -c node-14 -t 60 -b 500M 2>/dev/null || true
            sleep 1
        done &
        ;;

    "failed-systemd")
        echo "Setting up failed systemd units..."
        setup_systemctl_wrapper
        mkdir -p /etc/systemd/system

        cat > /etc/systemd/system/test-fail-1.service << 'EOF'
[Unit]
Description=Test Failing Service 1
After=network.target

[Service]
Type=oneshot
ExecStart=/bin/false
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

        cat > /etc/systemd/system/test-fail-2.service << 'EOF'
[Unit]
Description=Test Failing Service 2
After=network.target

[Service]
Type=oneshot
ExecStart=/bin/false
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

        ;;

    "offline-broken")
        echo "Starting broken node (will exit immediately)..."
        sleep 1
        exit 1
        ;;

    "normal"|*)
        echo "Starting normal idle node..."
        ;;
esac

exec /usr/sbin/sshd -D
