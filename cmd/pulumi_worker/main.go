package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/joho/godotenv"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	log.Println("test")

	globalVars := GlobalVars{
		TenantId:                       os.Getenv("TENANT_ID"),
		BillingScope:                   os.Getenv("BILLING_SCOPE"),
		ClientProjectManagementGroupId: os.Getenv("CLIENT_PROJ_MGMT_GROUP_ID"),
		Region:                         os.Getenv("REGION"),
	}

	projectOptions := template.Project{
		Name:             "hanta",
		Users:            map[template.Role][]template.User{},
		Groups:           map[template.Role]template.Group{},
		ServicePrincipal: template.ServicePrincipalOptions{},
		StorageAccount:   template.StorageAccountOptions{},
		KeyVault:         template.KeyVaultOptions{},
		DataFactory:      template.DataFactoryOptions{},
	}

	err = createUpdateStack(DEV, globalVars, projectOptions)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

}

func createUpdateStack(env Environment, globalVars GlobalVars, project template.Project) error {

	ctx := context.Background()
	stackName := env.String()
	projectName := fmt.Sprintf("client-project-%s", project.Name)

	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, runPulumiJob(DEV, project, globalVars))
	if err != nil {
		return err
	}
	fmt.Printf("Created/Selected stack %q\n", stackName)

	w := s.Workspace()

	fmt.Println("Installing the Azure Native plugin")

	err = w.InstallPlugin(ctx, "azure-native", "v3.19.0")
	if err != nil {
		fmt.Printf("Failed to install program plugins: %v\n", err)
		return err
	}

	fmt.Println("Successfully installed Azure Native plugin")

	s.SetConfig(ctx, "azure-native:location", auto.ConfigValue{Value: globalVars.Region})

	fmt.Println("Successfully set config")
	fmt.Println("Starting refresh")

	_, err = s.Refresh(ctx)
	if err != nil {
		fmt.Printf("Failed to refresh stack: %v\n", err)
		return err
	}

	fmt.Println("Refresh succeeded!")

	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	_, err = s.Up(ctx, stdoutStreamer)
	if err != nil {
		fmt.Printf("Failed to update stack: %v\n\n", err)
		return err
	}
	return nil
}
