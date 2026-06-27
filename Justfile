dev:
    go run ./cmd/api/*.go & go run ./cmd/pulumi_worker/*.go

proto:
    ls -d proto/**/*.proto | xargs -n 1 protoc -I=. --go_out=.

nats:
    nats-server -js -sd ./nats_data
