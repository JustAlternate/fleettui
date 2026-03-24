# FleetTUI

A TUI for monitoring your server fleet in real-time. Built with Go and the [Charm](https://charm.sh) stack.

![FleetTUI Demo](assets/demo.gif)

## Features

- **Real-time Monitoring**: Track CPU, RAM, network usage, uptime, and systemd status
- **Cards**: Each node displayed in a detailed card with progress bars and status indicators
- **List View**: Compact tabular view for monitoring many nodes at once
- **Interactive SSH Panel**: Open an in-app SSH terminal for the selected node (`S`)
- **Live Logs Panel**: Tail logs in-app (`L`) with follow/pause, filter, wrap mode, cursor navigation, and yank-to-clipboard
- **Built-in Search/Filter**: Filter nodes from the UI with `/` (supports negation via `!query`)
- **Parallel Collection**: Fetches metrics from all hosts concurrently using goroutines
- **Long-lived SSH Connections**: Reuses SSH connections for metrics collection
- **Configurable**: Enable/disable specific metrics via YAML configuration

## Installation

### Using Go

```bash
go install github.com/justalternate/fleettui
```

### Using Nix (with flakes)

```bash
nix run github:JustAlternate/fleettui
```

### From Source

```bash
git clone https://github.com/JustAlternate/fleettui.git
cd fleettui
go install .
```

## Usage

```bash
# Run with default config (~/.config/fleettui/)
fleettui

# Specify custom hosts file
fleettui -hosts ./hosts.yaml
```

### Keybindings

#### Global

- `q`: Quit
- `tab`: Switch cards/table view
- `/`: Open node filter
- `r`: Manual refresh
- `S`: Open SSH panel for selected node
- `L`: Open logs panel for selected node

#### Navigation

- `j/k`: Up/down
- `h/l`: Left/right (cards view)
- `g/G`: Top/bottom

#### SSH panel

- `ctrl+q`: Close panel

#### Logs panel

- `ctrl+q`: Close panel
- `f`: Toggle follow (streaming/pause)
- `w`: Toggle line wrap
- `c`: Clear buffered logs
- `/`: Filter logs
- `esc`: Exit/clear filter or exit selection mode
- `j/k`, `g/G`: Cursor/navigation
- `V`: Toggle multi-line selection
- `y`: Yank selected line(s) to clipboard

## Configuration

### Hosts Configuration (`hosts.yaml`)

```yaml
hosts:
  - name: server-01              # Display name (also used as IP if ip omitted)
    ip: 192.168.1.10             # IP address (default SSH port 22). Optional if name resolves.
    user: root                   # SSH user (default: root)
    ssh_key_path: ~/.ssh/id_rsa  # Optional: uses ssh-agent if not specified
  - name: server-02
    ip: 192.168.1.11:2222        # Custom SSH port supported
    user: admin
```

**Notes:**
- If `ip` is not specified, the `name` is used as the hostname/IP
- If `user` is not specified, defaults to `root`
- If `ssh_key_path` is not specified, uses `ssh-agent` if available
- Custom SSH port can be specified in `ip` field (e.g., `192.168.1.10:2222`)
- SSH host keys use Trust On First Use (TOFU) and are stored in `~/.ssh/known_hosts`

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

- Remote hosts must have SSH access and read access to `/proc` filesystem (standard on Linux)

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

## Inspiration

Inspired by [k9s](https://k9scli.io/), a terminal UI for Kubernetes clusters.
