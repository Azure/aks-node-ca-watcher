.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/aks-node-ca-watcher -v main.go
modules:
	go mod download