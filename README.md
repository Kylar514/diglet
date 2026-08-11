# diglet

![Go Version](https://img.shields.io/github/go-mod/go-version/kylar514/diglet)
![License](https://img.shields.io/github/license/kylar514/diglet)
[![Go Report Card](https://goreportcard.com/badge/github.com/kylar514/diglet)](https://goreportcard.com/report/github.com/kylar514/diglet)

TUI tunnel manager — start, stop, and monitor SSH, kubectl, and Docker
port-forward tunnels from your terminal.

<img width="807" height="605" alt="image" src="https://github.com/user-attachments/assets/0e6ed88d-2662-4163-ab21-1712854ab90f" />

## Use case?

- Forward ports from Kubernetes or remote servers for dev and troubleshooting.
  No one wants to remember 100 different hosts and ports, especially 6 months
  later when something breaks.

- What's a fella to do? Save your configs in connections.yaml and toggle away.
  Great for poking databases through Dadbod (especially behind a jump host).

- Tools like this can feel scary, you want to know what it's actually doing. So
  diglet shows a preview of the exact command. Even if you don't remember the
  syntax, it's one button press away.

## Install

```bash
go install github.com/kylar514/diglet@latest
```

Or build from source:

```bash
git clone https://github.com/kylar514/diglet.git
cd diglet
go build -o diglet .
```

## Usage

Run `diglet`. On first launch it scaffolds a config file in the platform's user
config directory (`~/.config/diglet/connections.yaml` on Linux and
`%AppData%\diglet\connections.yaml` on Windows).

| Key     | Action                   |
| ------- | ------------------------ |
| `j`/`k` | Navigate list            |
| `Enter` | Toggle tunnel on/off     |
| `/`     | Fuzzy-filter connections |
| `t`     | Filter by tunnel type    |
| `e`     | Edit config in `$EDITOR` |
| `K`     | Kill all tunnels         |
| `q`     | Quit                     |

## Config

Linux path: `~/.config/diglet/connections.yaml`

Windows path: `%AppData%\diglet\connections.yaml`

```yaml
connections:
  - name: my-server
    tunnel_type: ssh
    ssh_host: user@example.com
    local_port: 9000
    remote_port: 8080
  - name: my-pod
    tunnel_type: kubectl
    resource: deployment/my-app
    namespace: default
    local_port: 3000
    remote_port: 80
```

## Tunnel Types

- **SSH** — `ssh -L` reverse forwarding
- **kubectl** — `kubectl port-forward`
- **Docker** — _(not yet implemented)_

Adding a new type: create a file in `tunnel/` with an `init()` that calls
`tunnel.Register()`.

## Status from the CLI

`diglet status` prints the name of each active tunnel, one per line. Perfect
for waybar or any status bar:

```jsonc
"custom/diglet": {
  "exec": "diglet status",
  "interval": 2,
  "format": "{}"
}
```

Empty output = no tunnels = waybar hides the module. Exit code is 0 when any
tunnel is active, 1 when none are active, and 2 on an error.

## Tunnel state

Diglet reads the operating system's listener table to determine live status.
It stores only enough process metadata in the user cache directory to prove
that a process belongs to Diglet before stopping it. An unrelated process on a
configured port is shown as occupied and is never killed.

Linux status inspection requires `ss`. Windows uses the built-in `netstat`.
Both platforms require `ssh` or `kubectl` in `PATH` for their respective tunnel
types. Desktop notifications currently require `notify-send` and are therefore
Linux-only.

## License

MIT
