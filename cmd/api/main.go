package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
)

func main() {
	logger := log.New(os.Stdout, "[Broker API]: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	natsServer := nats.DefaultURL
	if os.Getenv("NATS_SERVER") != "" {
		natsServer = os.Getenv("NATS_SERVER")
	}
	ctx := context.Background()

	logger.Println("initializing connection to nats server: ", natsServer)
	nc, err := nats.Connect(natsServer)
	if err != nil {
		logger.Fatal("failed to connect to nats server: ", err)
	}
	defer nc.Close()

	logger.Println("upgrading nats connection to jetstream")
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("failed to upgrade nats connection to jetstream: ", err)
	}
	logger.Println("upgraded connection to jetstream")

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: "ProjectJobQueue", Description: "Stream to manage active jobs for projects", Subjects: []string{"create.*", "update.*", "delete.*"}})
	if err != nil {
		logger.Fatal("failed to create 'ProjectJobQueue' stream with error: ", err)
	}

	logger.Println("creating kvStore connection to deployments")
	kvStore, err := js.KeyValue(ctx, "deployments")
	if err != nil {
		logger.Fatalf("failed to select deployments kv store: %s", err)
	}

	port := 50051
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Fatalf("failed to listen on port %d with error %s\n", port, err)
	}
	s := grpc.NewServer()
	broker.RegisterBrokerServiceServer(s, &server{
		js:      js,
		kvStore: kvStore,
		logger:  logger,
	})

	logger.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}
}
