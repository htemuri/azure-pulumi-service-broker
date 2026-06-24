package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type NatsHandler struct {
	ctx     context.Context
	wg      *sync.WaitGroup
	envs    []Environment
	project *template.Project
	config  Config
}

func NewNatsHandler(ctx context.Context, wg *sync.WaitGroup, envs []Environment, project *template.Project, config Config) *NatsHandler {
	return &NatsHandler{
		ctx:     ctx,
		wg:      wg,
		envs:    envs,
		project: project,
		config:  config,
	}
}

func (nh *NatsHandler) configureWorkspace(stack *auto.Stack, env Environment) error {
	workspace := stack.Workspace()
	if env == Environment_ENTRA {
		err := workspace.InstallPlugin(nh.ctx, "azuread", nh.config.PulumiAzureADProviderVersion)
		if err != nil {
			return fmt.Errorf("failed to install program plugins: %v\n", err)
		}
		return nil
	}
	err := workspace.InstallPlugin(nh.ctx, "azure-native", nh.config.PulumiAzureNativeProviderVersion)
	if err != nil {
		return fmt.Errorf("failed to install program plugins: %v\n", err)
	}
	return nil
}

func (nh *NatsHandler) Handle() []error {
	errs := make(chan error, len(nh.envs))
	defer close(errs)

	for _, env := range nh.envs {
		nh.wg.Add(1)
		go func(e Environment) {
			err := nh.handleEnv(env)
			if err != nil {
				errs <- fmt.Errorf("failed to update project %s environment:\n\t%v", env.String(), err)
			} else {
				errs <- nil
			}
		}(env)
	}
	var errSlice []error
	for i := 0; i < len(nh.envs); i++ {
		err := <-errs
		if err != nil {
			errSlice = append(errSlice, err)
		}
	}
	return errSlice
}

func (nh *NatsHandler) handleEnv(env Environment) error {
	defer nh.wg.Done()
	handlerLogger := log.New(logger.Writer(), fmt.Sprintf("%s[%s] ", logger.Prefix(), env.String()), logger.Flags())
	// ctx := context.Background()
	projectName := fmt.Sprintf("client-project-%s", nh.project.Name)
	stackName := env.String()

	pj := PulumiJobs{env: env, project: nh.project, config: nh.config}

	var pulumiRunFunc pulumi.RunFunc

	if env == Environment_ENTRA {
		pulumiRunFunc = pj.EntraJob()
	} else {
		pulumiRunFunc = pj.ResourceJob()
	}

	s, err := auto.UpsertStackInlineSource(nh.ctx, stackName, projectName, pulumiRunFunc)
	if err != nil {
		return fmt.Errorf("failed to create/update stack with error: %s", err)
	}
	handlerLogger.Printf("created/selected stack %s/%s\n", projectName, s.Name())
	handlerLogger.Println("configuring workspace...")
	err = nh.configureWorkspace(&s, env)
	if err != nil {
		return fmt.Errorf("failed to configure stack workspace: %v\n", err)
	}

	s.SetConfig(nh.ctx, "azure-native:location", auto.ConfigValue{Value: nh.config.Region})

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

	// streamer := optup.ProgressStreams(handlerLogger.Writer())
	// streamer := optpreview.ProgressStreams(logger.Writer())
	_, err = s.Up(nh.ctx)
	if err != nil {
		return fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	handlerLogger.Printf("successfully updated %s env for project %s", env.String(), nh.project.Name)
	return nil
}
