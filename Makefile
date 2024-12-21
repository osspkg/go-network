
.PHONY: install
install:
	go install github.com/osspkg/devtool@latest

.PHONY: setup
setup:
	devtool setup-lib

.PHONY: lint
lint:
	devtool lint

.PHONY: license
license:
	devtool license

.PHONY: build
build:
	devtool build --arch=amd64

.PHONY: tests
tests:
	devtool test

.PHONY: pre-commite
pre-commite: setup lint build tests

.PHONY: ci
ci: install setup lint build tests


run_example_client_tcp:
	ADDRESS="127.0.0.1:8888" NETWORK="tcp" go run -race examples/client/main.go
run_example_server_tcp:
	ADDRESS="127.0.0.1:8888" NETWORK="tcp" go run -race examples/server/main.go

run_example_client_udp:
	ADDRESS="127.0.0.1:8888" NETWORK="udp" go run -race examples/client/main.go
run_example_server_udp:
	ADDRESS="127.0.0.1:8888" NETWORK="udp" go run -race examples/server/main.go

run_example_client_unix:
	ADDRESS="/tmp/unix.sock" NETWORK="unix" go run -race examples/client/main.go
run_example_server_unix:
	ADDRESS="/tmp/unix.sock" NETWORK="unix" go run -race examples/server/main.go

run_example_client_quic:
	ADDRESS="127.0.0.1:8888" NETWORK="quic" go run -race examples/client/main.go
run_example_server_quic:
	ADDRESS="127.0.0.1:8888" NETWORK="quic" go run -race examples/server/main.go

run_example_server_epoll:
	go run -race examples/epoll-server/main.go
