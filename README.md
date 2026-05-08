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

Run `diglet`. On first launch it scaffolds a config file at
`~/.config/diglet/connections.yaml`.

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

`~/.config/diglet/connections.yaml`:

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

## Session Recovery

Active tunnels are persisted to `/tmp/diglet-state.json` and automatically
restored on restart. Zombie processes are detected and cleaned up.

## License

MIT
