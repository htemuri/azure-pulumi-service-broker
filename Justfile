dev:
    go run ./cmd/api/*.go & go run ./cmd/pulumi_worker/*.go

proto:
    ls -d proto/**/*.proto | xargs -n 1 protoc -I=. --go_out=pkg/ --go_opt=module=github.com/htemuri/azure-pulumi-service-broker/pkg
nats:
    nats-server -js -sd ./nats_data
