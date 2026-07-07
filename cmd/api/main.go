package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/project"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func main() {
	logger := log.New(os.Stdout, "[Broker API]: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	natsServer := nats.DefaultURL
	ctx := context.Background()
	err := godotenv.Load()
	if err != nil {
		logger.Fatal("error loading .env file:", err)
	}

	logger.Println("initializing connection to nats server: ", natsServer)
	nc, err := nats.Connect(natsServer)
	if err != nil {
		logger.Fatal("failed to connect to nats server: ", err)
	}
	defer nc.Close()

	logger.Println("upgrading nats connection to jetstream")
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("failed to upgrade nats connection to jetstream: ", err)
	}
	logger.Println("upgraded connection to jetstream")

	// assume I unmarshalled grpc request to project protobuf go type
	projectName := "covid"
	env := templates.Environment_ENVIRONMENT_PRD
	reg := templates.Region_REGION_EASTUS
	pulumiCred := &templates.PulumiProviderCredentialArgs{
		TenantId:     os.Getenv("TENANT_ID"),
		ClientId:     os.Getenv("PULUMI_SP_CLIENT_ID"),
		ClientSecret: os.Getenv("PULUMI_SP_CLIENT_SECRET"),
	}
	defaultArgs := &templates.DefaultParams{
		ProjectName:              projectName,
		Environment:              env,
		Region:                   reg,
		PulumiProviderCredential: pulumiCred,
	}
	base := &templates.BaseRequest{
		DefaultParams: defaultArgs,
		Subscription: &templates.SubscriptionArgs{
			SubscriptionId: "23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c",
			// BillingScope:      os.Getenv("BILLING_SCOPE"),
			// ManagementGroupId: os.Getenv("CLIENT_PROJ_MGMT_GROUP_ID"),
		},
		VirtualNetwork: &templates.NetworkArgs{
			IpamPoolPrefixAllocations: &templates.IpamPoolPrefixAllocation{
				IpamPoolResourceId:  os.Getenv("CLIENT_PRD_IPAM_RESOURCE_ID"),
				NumberOfIpAddresses: 160},
			Subnets: []*templates.SubnetArgs{{Name: "default", NumberOfIpAddresses: 48}, {Name: "second", NumberOfIpAddresses: 32}}},
	}
	sec := &templates.SecurityRequest{
		DefaultParams: defaultArgs,
		KeyVault:      &templates.KeyVaultArgs{},
	}
	// stor, err := templates.NewStorageTemplate(
	// 	projectName, env, reg, pulumiCred,
	// )
	// if err != nil {
	// 	logger.Printf("failed to create storage template: %s", err)
	// }
	req := broker.CreateProjectRequest{
		Project: &project.Project{
			Name:        projectName,
			Environment: env,
		},
		TemplateRequests: []*templates.TemplatesRequest{
			{Request: &templates.TemplatesRequest_Base{Base: base}},
			{Request: &templates.TemplatesRequest_Security{Security: sec}},
			// {Template: &templates.Templates_Storage{Storage: stor}},
		},
	}
	// project := broker.Project{
	// 	Name:        projectName,
	// 	Environment: env,
	// 	Templates: []*templates.Templates{
	// 		{Template: &templates.Templates_Base{Base: base}},
	// 		{Template: &templates.Templates_Security{Security: sec}},
	// 		// {Template: &templates.Templates_Storage{Storage: stor}},
	// 	},
	// }

	// project := broker.Project{
	// 	Name:               "hanta",
	// 	Users:              []*broker.UserPersonaEntry{{Role: broker.RoleType_ROLE_TYPE_ADMIN, Users: []*broker.User{{UserPrincipalName: "productivity.catalyst766_slmail.me#EXT#@productivitycatalyst766slma.onmicrosoft.com", ObjectId: "b70d6761-f96e-4c7e-a352-b459099a3c09"}}}, {Role: broker.RoleType_ROLE_TYPE_DEVELOPER, Users: []*broker.User{{UserPrincipalName: "person1@productivitycatalyst766slma.onmicrosoft.com", ObjectId: "1cf052d6-aeed-4031-8fd3-aa857a3a6b29"}}}, {Role: broker.RoleType_ROLE_TYPE_READER, Users: []*broker.User{{UserPrincipalName: "person2@productivitycatalyst766slma.onmicrosoft.com", ObjectId: "56752270-5c9a-488f-a398-62e7c3108b30"}}}},
	// 	Groups:             make([]*broker.GroupPersonaEntry, 0),
	// 	ServicePrincipal:   &broker.ServicePrincipalOptions{Enabled: false},
	// 	StorageAccount:     &broker.StorageAccountOptions{Enabled: true, SubResources: []broker.StorageAccountSubResource{}},
	// 	KeyVaultOptions:    &broker.KeyVaultOptions{Enabled: false},
	// 	DataFactoryOptions: &broker.DataFactoryOptions{Enabled: false},
	// }
	// _, err = templates.GetEnabledTemplates(project.Templates)
	// if err != nil {
	// 	logger.Print(err)
	// 	log.Default().Fatal(err)
	// }

	dataBytes, err := proto.Marshal(&req) // cant error from a generated protobuf go type
	if err != nil {
		logger.Fatalf("failed to marshal project: %s", err)
	}
	// create/update the project stream

	_, err = js.UpdateStream(ctx, jetstream.StreamConfig{Name: "ProjectJobQueue", Description: "Stream to manage active jobs for projects", Subjects: []string{"create", "update", "delete", "failed", "success"}})
	if err != nil {
		logger.Fatal("failed to create 'ProjectJobQueue' stream with error: ", err)
	}
	_, err = js.Publish(ctx, "create", dataBytes)
	if err != nil {
		logger.Println("failed to publish to subject 'create' with error: ", err)
	}

	// check success subject
	var wg sync.WaitGroup
	wg.Add(1)

	consumer, err := js.CreateOrUpdateConsumer(ctx, "ProjectJobQueue", jetstream.ConsumerConfig{
		Name: "broker_api", Durable: "broker_api",
	})
	if err != nil {
		logger.Fatalf("failed to create/update durable consumer against %s stream with error: %s\n", "ProjectJobQueue", err)
	}

	if _, err := consumer.Consume(func(msg jetstream.Msg) {
		if msg.Subject() == "success" {
			var pr broker.CreateProjectResponse
			err := proto.Unmarshal(msg.Data(), &pr)
			if err != nil {
				logger.Printf("failed to unmarshal projectResponse: %s\n", err)
				return
			}
			logger.Printf("Project Response: %v\n", &pr)
			msg.Ack()
		}
	}); err != nil {
		logger.Fatal("failed to consume messages from durable stream with error:", err)
	}

	wg.Wait()

}
