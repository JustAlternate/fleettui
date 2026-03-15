# FleetTUI

A TUI for monitoring your server fleet in real-time. Built with Go and the [Charm](https://charm.sh) stack.

![FleetTUI Demo](assets/demo.gif)

## Features

- **Real-time Monitoring**: Track CPU, RAM, network usage, uptime, and systemd status
- **Cards**: Each node displayed in a detailed card with progress bars and status indicators
- **Parallel Collection**: Fetches metrics from all hosts concurrently using goroutines
- **Configurable**: Enable/disable specific metrics via YAML configuration

## Installation

### From Source

```bash
git clone https://github.com/JustAlternate/fleettui.git
cd fleettui
go install .
```

### Using Nix (with flakes)

```bash
nix run github:JustAlternate/fleettui
```

## Usage

```bash
# Run with default config (~/.config/fleettui/)
fleettui

# Specify custom hosts file
fleettui -hosts ./hosts.yaml
```

## Configuration

### Hosts Configuration (`hosts.yaml`)

```yaml
hosts:
  - name: server-01              # Display name
    ip: 192.168.1.10             # IP address (default SSH port 22)
    user: root                   # SSH user (default: root)
    ssh_key_path: ~/.ssh/id_rsa  # Optional: specific SSH key
  - name: server-02
    ip: 192.168.1.11:2222        # Custom SSH port supported
    user: admin
```

**Notes:**
- If `user` is not specified, defaults to `root`
- If `ssh_key_path` is not specified, uses the first key found in `~/.ssh/`
- Custom SSH port can be specified in `ip` field (e.g., `192.168.1.10:2222`)

### Application Configuration (`config.yaml`)

```yaml
refresh_rate: 5s              # How often to refresh metrics
metrics:                      # Enabled metrics
  - cpu                       # CPU usage percentage
  - ram                       # RAM usage percentage
  - network                   # Network I/O rates (MB/s)
  - connectivity              # SSH connectivity status
  - uptime                    # System uptime
  - systemd                   # Failed systemd units
  - os                        # OS name from /etc/os-release
```

## Requirements

- Remote hosts must have standard Linux utilities (`top`, `free`, `cat`, `systemctl`)

## Development

```bash
# Clone and build
git clone https://github.com/JustAlternate/fleettui.git
cd fleettui
go build .
```

## Testing

Run all tests:
```bash
go test ./...
```

Run with coverage:
```bash
go test -cover ./...
```

### Generating Mocks

When interface changes require new mocks, run:
```bash
mockery --all
```

## License

MIT License.
