proto:
    ls -d proto/**/*.proto | xargs -n 1 protoc -I=. --go_out=pkg/ --go_opt=module=github.com/htemuri/azure-pulumi-service-broker/pkg --go-grpc_out=pkg/ --go-grpc_opt=module=github.com/htemuri/azure-pulumi-service-broker/pkg
nats:
    nats-server -js -c docker/nats.conf
