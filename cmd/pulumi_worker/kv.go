package main

import (
	"context"
	"fmt"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func updateProjectStatusKV(ctx context.Context, kvStore jetstream.KeyValue, deploymentId string, projectStatus *broker.GetProjectStatusResponse) error {
	dataBytes, err := proto.Marshal(projectStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal projectStatus: %s", err)

	}
	_, err = kvStore.Put(ctx, deploymentId, dataBytes)
	if err != nil {
		return fmt.Errorf("failed to update kv store with error: %s\n", err)
	}
	return nil
}
