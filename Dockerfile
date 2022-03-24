FROM golang:latest as builder

COPY . /aks-node-ca-watcher
WORKDIR /aks-node-ca-watcher
RUN make modules
RUN make build

#second stage
FROM scratch
WORKDIR /
COPY --from=builder /aks-node-ca-watcher/bin/aks-node-ca-watcher /aks-node-ca-watcher
CMD ["/aks-node-ca-watcher"]