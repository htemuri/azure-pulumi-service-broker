package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/joho/godotenv"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

func templateTest() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file:", err)
	}
	pulumiProject := "test-proj-template"
	projectName := "test-template"
	env := templates.Environment_ENVIRONMENT_DEV
	base, err := templates.NewBaseTemplate(
		projectName, env, templates.Region_REGION_EASTUS, &templates.SubscriptionArgs{
			BillingScope:      os.Getenv("BILLING_SCOPE"),
			ManagementGroupId: os.Getenv("CLIENT_PROJ_MGMT_GROUP_ID"),
		}, &templates.NetworkArgs{IpamPoolPrefixAllocations: &templates.IpamPoolPrefixAllocation{IpamPoolResourceId: os.Getenv("CLIENT_DEV_IPAM_RESOURCE_ID"), NumberOfIpAddresses: 32}, Subnets: make([]*templates.SubnetArgs, 0)}, &templates.PulumiProviderCredentialArgs{
			TenantId:     os.Getenv("TENANT_ID"),
			ClientId:     os.Getenv("PULUMI_SP_CLIENT_ID"),
			ClientSecret: os.Getenv("PULUMI_SP_CLIENT_SECRET"),
		})
	if err != nil {
		log.Fatal(err)
	}
	_ = broker.Project{
		Name:        projectName,
		Environment: env,
		Templates: &templates.Templates{
			Base: base,
		},
	}
	ctx := context.Background()
	s, err := auto.UpsertStackInlineSource(ctx, base.DefaultParams.StackName, pulumiProject, base.PulumiRunFunc())
	if err != nil {
		log.Fatalf("failed to create/update stack with error: %s", err)
	}
	fmt.Printf("created/selected stack %s/%s\n", projectName, s.Name())
	fmt.Println("configuring workspace...")
	workspace := s.Workspace()
	for _, p := range base.DefaultParams.Providers {
		err := workspace.InstallPlugin(ctx, p.ProviderName, p.Version)
		if err != nil {
			log.Fatalf("failed to install program plugins: %v\n", err)
		}
	}

	s.SetConfig(ctx, "azure-native:location", auto.ConfigValue{Value: base.DefaultParams.Environment.ShortString()})

	for k, v := range autonamingConfig {
		c := fmt.Sprintf("pulumi:autonaming.providers.azure-native.resources.azure-native:%s.pattern", k)
		err = s.SetConfigWithOptions(ctx, c, auto.ConfigValue{Value: v}, &auto.ConfigOptions{
			Path: true,
		})
		if err != nil {
			log.Fatalf("failed to set autonaming config: %v\n", err)
		}
	}

	_, err = s.Refresh(ctx)
	if err != nil {
		log.Fatalf("failed to refresh stack: %v\n", err)
	}

	logLevel := uint(1) // 1 - 11 (least verbose to most verbose)

	debugOpts := debug.LoggingOptions{
		LogToStdErr:   true,
		LogLevel:      &logLevel,
		FlowToPlugins: true,
		Debug:         true,
	}

	streamer := optup.ProgressStreams(os.Stdout)
	_, err = s.Up(ctx, optup.DebugLogging(debugOpts), streamer)

	// streamer := optup.ProgressStreams(handlerLogger.Writer())
	// _, err = s.Preview(ctx, streamer)
	if err != nil {
		log.Fatalf("failed to update stack: %v\n\n", err)
	}
	fmt.Printf("successfully updated %s env for project %s", env.ShortString(), pulumiProject)
}

func repoTest() {
	repoUrl := "https://github.com/pulumi/templates"
	// projectName := "azure-go"
	pulumiProject := "test-remote-repo-2"

	_ = auto.GitRepo{
		URL: repoUrl,
		// ProjectPath: projectName,
		Setup: func(ctx context.Context, workspace auto.Workspace) error {
			workspace.New(ctx, &auto.NewOptions{
				TemplateOrURL: "https://github.com/pulumi/templates/tree/master/azure-go",
				Name:          pulumiProject,
			})
			curr, err := workspace.ProjectSettings(ctx)
			if err != nil {
				fmt.Println("failed to get project settings with error:", err)
				return err
			}
			curr.Name = "test-remote-repo"
			// cmd := exec.Command()
			return workspace.SaveProjectSettings(ctx, curr)
		},
	}
	// w := auto.Repo(r)

	ctx := context.Background()
	w, err := auto.NewLocalWorkspace(ctx)
	if err != nil {
		fmt.Println("failed to create new local workspace", err)
		os.Exit(1)
	}
	w.New(ctx, &auto.NewOptions{
		TemplateOrURL: "https://github.com/pulumi/templates/tree/master/azure-go",
		Name:          pulumiProject,
		Stack:         "test",
		Force:         true,
		TemplateMode:  true,
		// GenerateOnly:  true,
		// TemplateMode: true,
	})
	stackName := auto.FullyQualifiedStackName("htemuri", pulumiProject, "test")
	// fmt.Println(stackName)
	s, err := auto.UpsertStack(ctx, stackName, w)

	// s, err := auto.UpsertStackRemoteSource(ctx, "test", r)
	if err != nil {

		fmt.Printf("Failed to create or select stack: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created/Selected stack %q, and cloned program from git\n", s.Name())

	// set stack configuration specifying the AWS region to deploy
	s.SetConfig(ctx, "azure-native:location", auto.ConfigValue{Value: "EastUS"})

	fmt.Println("Successfully set config")
	fmt.Println("Starting refresh")

	_, err = s.Refresh(ctx)
	if err != nil {
		fmt.Printf("Failed to refresh stack: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Refresh succeeded!")

	fmt.Println("Starting update")

	// wire up our update to stream progress to stdout
	stdoutStreamer := optpreview.ProgressStreams(os.Stdout)

	// run the update to deploy our s3 website
	_, err = s.Preview(ctx, stdoutStreamer)
	if err != nil {
		fmt.Printf("Failed to update stack: %v\n\n", err)
		os.Exit(1)
	}

	fmt.Println("Update succeeded!")

}
