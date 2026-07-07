package main

import (
	"context"
	"fmt"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/project"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

type NatsHandler struct {
	ctx              context.Context
	project          *project.Project
	templateRequests []*templates.TemplatesRequest
}

func NewNatsHandler(ctx context.Context, project *project.Project, templateRequests []*templates.TemplatesRequest) *NatsHandler {
	return &NatsHandler{
		ctx:              ctx,
		project:          project,
		templateRequests: templateRequests,
	}
}

func (nh *NatsHandler) Handle() ([]*templates.TemplatesResponse, error) {
	var responses []*templates.TemplatesResponse
	var _templates []templates.Template
	for _, tr := range nh.templateRequests {
		t, err := tr.NewTemplate()
		if err != nil {
			return responses, fmt.Errorf("failed to convert template request to template: %s", err)
		}
		_templates = append(_templates, t)
	}

	orderedTemplates, err := templates.GetTemplateInstallOrder(_templates)
	if err != nil {
		return responses, fmt.Errorf("failed to get template install order: %s", err)
	}

	// 1 - 11 (least verbose to most verbose)
	logLevel := uint(1)

	debugLogging := optup.DebugLogging(debug.LoggingOptions{
		LogToStdErr:   true,
		LogLevel:      &logLevel,
		FlowToPlugins: true,
		Debug:         false,
	})
	streamer := optup.ProgressStreams(os.Stdout)

	for _, t := range orderedTemplates {
		logger.Printf("deploying %T template...", t)
		res, err := t.Deploy(nh.ctx, responses, autonamingConfig, debugLogging, streamer)
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
