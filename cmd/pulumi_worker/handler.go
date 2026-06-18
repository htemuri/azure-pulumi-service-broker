package main

import (
	"context"
	"fmt"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

func handleProjectEntraUpdate(config Config, project *template.Project) error {
	ctx := context.Background()
	stackName := "entra"
	projectName := fmt.Sprintf("client-project-%s", project.Name)

	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, runPulumiEntraJob(project, config))
	if err != nil {
		return fmt.Errorf("failed to create/update stack with error: %s", err)
	}
	logger.Printf("created/selected stack %s/%s\n", projectName, s.Name())
	logger.Println("configuring workspace...")
	w := s.Workspace()

	err = w.InstallPlugin(ctx, "azuread", "v6.9.1")
	if err != nil {
		return fmt.Errorf("failed to install program plugins: %v\n", err)
	}
	_, err = s.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh stack: %v\n", err)
	}

	streamer := optup.ProgressStreams(logger.Writer())

	_, err = s.Up(ctx, streamer)
	if err != nil {
		return fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	logger.Println("successfully provisioned/updated entra objects for project ", project.Name)
	return nil
}

func handleProjectResourceUpdate(env Environment, config Config, project *template.Project) error {

	ctx := context.Background()
	stackName := env.String()
	projectName := fmt.Sprintf("client-project-%s", project.Name)

	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, runPulumiResourceJob(env, project, config))
	if err != nil {
		return fmt.Errorf("failed to create/update stack with error: %s", err)
	}
	logger.Printf("created/selected stack %s/%s\n", projectName, s.Name())
	logger.Println("configuring workspace...")
	w := s.Workspace()

	err = w.InstallPlugin(ctx, "azure-native", "v3.19.0")
	if err != nil {
		return fmt.Errorf("failed to install program plugins: %v\n", err)
	}

	s.SetConfig(ctx, "azure-native:location", auto.ConfigValue{Value: config.Region})

	for k, v := range autonamingConfig {
		c := fmt.Sprintf("pulumi:autonaming.providers.azure-native.resources.azure-native:%s.pattern", k)
		err = s.SetConfigWithOptions(ctx, c, auto.ConfigValue{Value: v}, &auto.ConfigOptions{
			Path: true,
		})
		if err != nil {
			return fmt.Errorf("failed to set autonaming config: %v\n", err)
		}
	}

	_, err = s.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh stack: %v\n", err)
	}

	streamer := optup.ProgressStreams(logger.Writer())
	// streamer := optpreview.ProgressStreams(logger.Writer())
	_, err = s.Up(ctx, streamer)
	if err != nil {
		return fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	logger.Printf("successfully provisioned/updated %s resources for project %s", env.String(), project.Name)
	return nil
}
