package main

import (
	"context"
	"fmt"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
)

type NatsHandler struct {
	ctx              context.Context
	project          *broker.Project
	templateRequests []*templates.Templates
}

func NewNatsHandler(ctx context.Context, project *broker.Project, templateRequests []*templates.Templates) *NatsHandler {
	return &NatsHandler{
		ctx:              ctx,
		project:          project,
		templateRequests: templateRequests,
	}
}

func (nh *NatsHandler) Handle() ([]*templates.TemplatesResponse, error) {
	var responses []*templates.TemplatesResponse
	_templates, err := templates.GetTemplateInstallOrder(nh.templateRequests)
	// logger.Println("templates to deploy", _templates)
	if err != nil {
		return responses, fmt.Errorf("failed to get template install order: %s", err)
	}
	for _, t := range _templates {
		logger.Printf("deploying %T template...", t)
		res, err := t.Deploy(nh.ctx, responses, autonamingConfig)
		if err != nil {
			responses = append(responses, &templates.TemplatesResponse{
				Status: templates.Status_STATUS_ERROR,
				Error:  fmt.Sprintf("failed deploying template: %s", err),
			})
			return responses, fmt.Errorf("failed deploying template: %s", err)
		}
		responses = append(responses, &templates.TemplatesResponse{
			Status:   templates.Status_STATUS_SUCCESS,
			Response: res,
		})
	}
	return responses, nil
}
