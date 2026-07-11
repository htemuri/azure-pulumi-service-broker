package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/broker"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/project"
	"github.com/htemuri/azure-pulumi-service-broker/pkg/templates"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := log.New(os.Stdout, "[Broker Client]: ", log.Ldate|log.Ltime|log.Lmsgprefix)

	serverHost := "localhost"
	serverPort := 50051
	serverAddr := fmt.Sprintf("%s:%d", serverHost, serverPort)

	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("failed to connect to server: %s", err)
	}
	defer conn.Close()
	client := broker.NewBrokerServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = godotenv.Load()
	if err != nil {
		logger.Fatalf("error loading .env file: %s", err)
	}

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
			Subnets: []*templates.SubnetArgs{
				{Name: "default", NumberOfIpAddresses: 48},
				{Name: "second", NumberOfIpAddresses: 32}}},
	}
	sec := &templates.SecurityRequest{
		DefaultParams: defaultArgs,
		KeyVault: &templates.KeyVaultArgs{
			NetworkSettings: &templates.ResourceNetworkArgs{
				PrivateEndpoint: &templates.PrivateEndpointArgs{
					Enabled:      true,
					SubResources: []string{"vault"},
				},
			},
		},
	}
	stor := &templates.StorageRequest{
		DefaultParams: defaultArgs,
		StorageAccount: &templates.StorageAccountArgs{
			HnsEnabled: true,
			Kind:       templates.StorAcctKind_STOR_ACCT_KIND_STORAGE_V2,
			Sku:        templates.StorAcctSKU_STOR_ACCT_SKU_STANDARD_LRS,
			NetworkSettings: &templates.ResourceNetworkArgs{
				PrivateEndpoint: &templates.PrivateEndpointArgs{
					Enabled:      true,
					SubResources: []string{"blob", "dfs"},
				},
			},
		},
	}

	reqInput := broker.CreateProjectRequest{
		Project: &project.Project{
			Name:        projectName,
			Environment: env,
		},
		TemplateRequests: []*templates.TemplatesRequest{
			{Request: &templates.TemplatesRequest_Base{Base: base}},
			{Request: &templates.TemplatesRequest_Security{Security: sec}},
			{Request: &templates.TemplatesRequest_Storage{Storage: stor}},
		},
	}

	logger.Println("sending project deployment request")
	res, err := client.CreateProject(ctx, &reqInput)
	if err != nil {
		logger.Fatalf("could not deploy project request: %s", err)
	}
	logger.Printf("deployment id: %v", res.GetDeploymentId())

	for range 45 { // check status for 7.5 minutes
		time.Sleep(time.Second * 10) // every 10 seconds
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		stat, err := client.GetProjectStatus(ctx, &broker.GetProjectStatusRequest{
			DeploymentId: res.GetDeploymentId(),
		})
		if err != nil {
			logger.Fatalf("failed to get status of deployment %s", err)
		}
		if stat.GetStatus() == templates.Status_STATUS_SUCCESS {
			bytes, _ := json.MarshalIndent(stat, "", "  ")
			logger.Printf("completed deployment: \n%v", string(bytes))
			return
		}
		if stat.GetStatus() == templates.Status_STATUS_ERROR {
			logger.Printf("failed deployment: %s", stat.Error)
			return
		}
		logger.Printf("current status: %v", stat.GetStatus())
	}
}
