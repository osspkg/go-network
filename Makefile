SHELL=/bin/bash


.PHONY: install
install:
	go install go.osspkg.com/goppy/v2/cmd/goppy@latest
	goppy setup-lib

.PHONY: lint
lint:
	goppy lint

.PHONY: license
license:
	goppy license

.PHONY: build
build:
	goppy build --arch=amd64

.PHONY: tests
tests:
	goppy test

.PHONY: pre-commit
pre-commit: install license lint tests build

.PHONY: ci
ci: pre-commit

run_example_client_tcp:
	time ADDRESS="127.0.0.1:8888" NETWORK="tcp" go run -race examples/client/main.go
run_example_server_tcp:
	ADDRESS="127.0.0.1:8888" NETWORK="tcp" go run -race examples/server/main.go

run_example_client_udp:
	time ADDRESS="127.0.0.1:8888" NETWORK="udp" go run -race examples/client/main.go
run_example_server_udp:
	ADDRESS="127.0.0.1:8888" NETWORK="udp" go run -race examples/server/main.go

run_example_client_unix:
	time ADDRESS="/tmp/unix.sock" NETWORK="unix" go run -race examples/client/main.go
run_example_server_unix:
	ADDRESS="/tmp/unix.sock" NETWORK="unix" go run -race examples/server/main.go

run_example_client_quic:
	time ADDRESS="127.0.0.1:8888" NETWORK="quic" go run -race examples/client/main.go
run_example_server_quic:
	ADDRESS="127.0.0.1:8888" NETWORK="quic" go run -race examples/server/main.go

run_example_server_epoll:
	go run -race examples/epoll-server/main.go
