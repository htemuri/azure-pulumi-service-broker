package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

func main() {
	logger := log.New(os.Stdout, "[Pulumi Worker]: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	natsServer := nats.DefaultURL
	ctx := context.Background()

	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file:", err)
	}

	logger.Println("initializing connection to nats server:", natsServer)
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Fatal("failed to connect to nats server:", err)
	}
	defer nc.Close()

	logger.Println("upgrading nats connection to jetstream")
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("failed to upgrade nats connection to jetstream: ", err)
	}
	logger.Println("upgraded connection to jetstream")

	// wg := sync.WaitGroup{}
	// wg.Add(1)

	kv, err := js.KeyValue(ctx, "hanta")
	if err != nil {
		logger.Fatal("failed to bind to kv store 'hanta' with error: ", err)
	}
	watcher, err := kv.Watch(ctx, "name")
	defer watcher.Stop()
	if err != nil {
		logger.Fatal("failed to get value for key 'name' with error: ", err)
	}
	// watcher.Stop()
	for {
		select {
		case val := <-watcher.Updates():
			if val == nil {
				continue
			}
			logger.Printf("Key: %s, Operation: %v, Value: %s\n", val.Key(), val.Operation(), string(val.Value()))

		}
	}
	// for val := range watcher.Updates() {
	// 	logger.Print("value for key 'hanta' is: ", string(val.Value()))
	// 	// watcher.Stop()
	// }

	// logger.Println("subscribing to updates to subject:", natsSubject)
	// if _, err := nc.Subscribe(natsSubject, func(m *nats.Msg) {
	// 	logger.Printf("received data: %s\n", string(m.Data))
	// }); err != nil {
	// 	logger.Fatalf("error receiving message from subject [%s]: %s", natsSubject, err)
	// }

	// keep waiting for nats messages
	// wg.Wait()
	// defer watcher.Stop()

	// globalVars := GlobalVars{
	// 	TenantId:                       os.Getenv("TENANT_ID"),
	// 	BillingScope:                   os.Getenv("BILLING_SCOPE"),
	// 	ClientProjectManagementGroupId: os.Getenv("CLIENT_PROJ_MGMT_GROUP_ID"),
	// 	Region:                         os.Getenv("REGION"),
	// }

	// projectOptions := template.Project{
	// 	Name:             "hanta",
	// 	Users:            map[template.Role][]template.User{},
	// 	Groups:           map[template.Role]template.Group{},
	// 	ServicePrincipal: template.ServicePrincipalOptions{},
	// 	StorageAccount:   template.StorageAccountOptions{},
	// 	KeyVault:         template.KeyVaultOptions{},
	// 	DataFactory:      template.DataFactoryOptions{},
	// }

	// err = createUpdateStack(DEV, globalVars, projectOptions)
	// if err != nil {
	// 	log.Print(err)
	// 	os.Exit(1)
	// }

}

func handleProjectUpdate() {}

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
