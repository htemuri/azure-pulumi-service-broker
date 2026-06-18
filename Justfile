dev:
    go run ./cmd/api/*.go & go run ./cmd/pulumi_worker/*.go

proto:
    protoc -I=. --go_out=. proto/project.proto

nats:
    nats-server -js -sd ./nats_data