package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type server struct {
	js      jetstream.JetStream
	kvStore jetstream.KeyValue
	logger  *log.Logger
	broker.UnimplementedBrokerServiceServer
}

func (s *server) CreateProject(ctx context.Context, req *broker.CreateProjectRequest) (*broker.CreateProjectResponse, error) {
	deploymentId := uuid.New().String()
	s.logger.Printf("recieved request to deploy project '%s'", req.Project.GetName())

	dataBytes, err := proto.Marshal(req)
	if err != nil {
		return &broker.CreateProjectResponse{}, fmt.Errorf("failed to marshal project: %s", err)
	}

	_, err = s.js.Publish(ctx, fmt.Sprintf("create.%s", deploymentId), dataBytes)
	if err != nil {
		return &broker.CreateProjectResponse{}, fmt.Errorf("failed to publish to subject 'create': %s", err)
	}

	return &broker.CreateProjectResponse{
		DeploymentId: deploymentId,
		ProjectName:  req.GetProject().GetName(),
		Status:       templates.Status_STATUS_IN_PROGRESS,
		Error:        "",
	}, nil

}

func (s *server) GetProjectStatus(ctx context.Context, req *broker.GetProjectStatusRequest) (*broker.GetProjectStatusResponse, error) {
	var res broker.GetProjectStatusResponse
	s.logger.Printf("recieved request to get status of deploymentId '%s'", req.GetDeploymentId())
	deploymentId := req.GetDeploymentId()
	if deploymentId == "" {
		return &broker.GetProjectStatusResponse{}, fmt.Errorf("deploymentId cannot be empty")
	}
	data, err := s.kvStore.Get(ctx, req.GetDeploymentId())
	if err != nil {
		return &broker.GetProjectStatusResponse{}, fmt.Errorf("failed getting deploymentId from kv: %s", err)
	}
	err = proto.Unmarshal(data.Value(), &res)
	if err != nil {
		return &broker.GetProjectStatusResponse{}, fmt.Errorf("failed to unmarshal kv store value to response: %s", err)
	}
	return &res, nil
}
