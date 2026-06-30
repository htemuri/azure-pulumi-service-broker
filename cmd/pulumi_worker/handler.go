package main

import (
	"context"
	"fmt"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

type NatsHandler struct {
	ctx     context.Context
	project *broker.Project
}

func NewNatsHandler(ctx context.Context, project *broker.Project) *NatsHandler {
	return &NatsHandler{
		ctx:     ctx,
		project: project,
	}
}

func (nh *NatsHandler) Handle() error {
	templates, err := templates.GetTemplateInstallOrder(nh.project.Templates)
	if err != nil {
		return fmt.Errorf("failed to get template install order: %s", err)
	}
	for _, t := range templates {
		err = nh.handleTemplate(t)
		if err != nil {
			return fmt.Errorf("failed deploying template: %s", err)
		}
	}
	return nil
}

func (nh *NatsHandler) handleTemplate(t templates.Template) error {
	projectName := fmt.Sprintf("client-project-%s", nh.project.Name)

	s, err := auto.UpsertStackInlineSource(nh.ctx, t.GetDefaultParams().StackName, projectName, t.PulumiRunFunc())
	if err != nil {
		return fmt.Errorf("failed to create/update stack with error: %s", err)
	}
	logger.Printf("created/selected stack %s/%s\n", projectName, s.Name())
	logger.Println("configuring workspace...")
	workspace := s.Workspace()
	for _, p := range t.GetDefaultParams().GetProviders() {
		err := workspace.InstallPlugin(nh.ctx, p.ProviderName, p.Version)
		if err != nil {
			return fmt.Errorf("failed to install program plugins: %v\n", err)
		}
	}
	s.SetConfig(nh.ctx, "azure-native:location", auto.ConfigValue{Value: t.GetDefaultParams().Region.ShortString()})
	for k, v := range autonamingConfig {
		c := fmt.Sprintf("pulumi:autonaming.providers.azure-native.resources.azure-native:%s.pattern", k)
		err = s.SetConfigWithOptions(nh.ctx, c, auto.ConfigValue{Value: v}, &auto.ConfigOptions{
			Path: true,
		})
		if err != nil {
			return fmt.Errorf("failed to set autonaming config: %v\n", err)
		}
	}

	_, err = s.Refresh(nh.ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh stack: %v\n", err)
	}

	// 1 - 11 (least verbose to most verbose)
	logLevel := uint(1)

	debugOpts := debug.LoggingOptions{
		LogToStdErr:   true,
		LogLevel:      &logLevel,
		FlowToPlugins: true,
		Debug:         true,
	}

	streamer := optup.ProgressStreams(os.Stdout)
	_, err = s.Up(nh.ctx, optup.DebugLogging(debugOpts), streamer)
	if err != nil {
		return fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	logger.Printf("successfully updated %s env for project %s", t.GetDefaultParams().Environment.ShortString(), nh.project.Name)
	return nil
}
