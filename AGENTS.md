# Agent instructions

## Repository

- Go module: `go.osspkg.com/network`; supported Go version: 1.26.
- Public packages: `cert`, `client`, `epoll`, `fd`, `listen`, `proto/udpframe`, `server`, and `xnet`.
- Runnable programs are in `examples/client`, `examples/server`, and `examples/epoll-server`.
- `epoll` uses Linux epoll for TCP and has a non-Linux stub. Keep Linux-specific syscalls behind its existing build constraints.

## Commands

Run commands from the repository root.

- Focused tests: `go test ./<package>`; full Go test suite: `go test ./...`.
- `make tests` runs `goppy test`.
- `make lint` runs `goppy lint`. It runs `go mod tidy`, `go generate`, `goimports`, `gofmt`, golangci-lint, and govulncheck; expect it to modify formatting, generated files, and module metadata.
- `make build` runs `goppy build --arch=amd64`.
- `make pre-commit` runs license, lint, tests, and build checks.
- CI runs `make ci`, which installs and sets up `goppy` before running pre-commit checks.

The CI workflow tests with Go 1.26 on Ubuntu. The `run_example_server_epoll` Make target is Linux-specific.
