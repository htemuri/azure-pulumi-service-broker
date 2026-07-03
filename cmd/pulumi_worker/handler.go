package main

import (
	"context"
	"fmt"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
)

type NatsHandler struct {
	ctx     context.Context
	cm      map[string]any
	project *broker.Project
}

func NewNatsHandler(ctx context.Context, project *broker.Project) *NatsHandler {
	return &NatsHandler{
		ctx:     ctx,
		cm:      map[string]any{},
		project: project,
	}
}

func (nh *NatsHandler) Handle() (map[string]any, error) {
	templates, err := templates.GetTemplateInstallOrder(nh.project.Templates)
	if err != nil {
		return nil, fmt.Errorf("failed to get template install order: %s", err)
	}
	for _, t := range templates {
		logger.Printf("deploying %T template...", t)
		cm, err := t.Deploy(nh.ctx, nh.cm, autonamingConfig)
		if err != nil {
			return nil, fmt.Errorf("failed deploying template: %s", err)
		}
		nh.cm = cm
	}
	return nh.cm, nil
}
