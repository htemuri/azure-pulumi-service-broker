package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
)

func repoTest() {
	repoUrl := "https://github.com/pulumi/templates"
	projectName := "azure-go"
	r := auto.GitRepo{
		URL:         repoUrl,
		ProjectPath: projectName,
		Setup: func(ctx context.Context, workspace auto.Workspace) error {
			curr, err := workspace.ProjectSettings(ctx)
			if err != nil {
				fmt.Println("failed to get project settings with error:", err)
				return err
			}
			curr.Name = "test-remote-repo"
			return workspace.SaveProjectSettings(ctx, curr)
		},
	}
	// w := auto.Repo(r)

	// pulumiProject := "test-remote-repo"
	ctx := context.Background()
	stackName := auto.FullyQualifiedStackName("htemuri", projectName, "test")
	fmt.Println(stackName)

	s, err := auto.UpsertStackRemoteSource(ctx, "test", r)
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
