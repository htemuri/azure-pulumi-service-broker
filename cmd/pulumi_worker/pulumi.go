package main

import (
	"fmt"
	"strings"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk"
	"github.com/pulumi/pulumi-azure-native-sdk/keyvault"
	"github.com/pulumi/pulumi-azure-native-sdk/network"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/storage"
	"github.com/pulumi/pulumi-azure-native-sdk/subscription/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Hard coding the name of the resource logical names because if you wish to change your resource types' physical
// naming scheme, pulumi wont see it as a whole new resource. Should update the config of the existing resource
// unless the name can't be edited in-place

func runPulumiJob(env Environment, project *template.Project, config Config) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		sub, err := subscription.NewAlias(ctx, "subscription", &subscription.AliasArgs{
			Properties: subscription.PutAliasRequestPropertiesArgs{
				DisplayName:  pulumi.String(fmt.Sprintf("[%s] Client Project: %s", strings.ToUpper(env.String()), strings.ToUpper(project.Name))),
				BillingScope: pulumi.String(config.BillingScope),
				Workload:     pulumi.String("Production"),
				AdditionalProperties: subscription.PutAliasRequestAdditionalPropertiesArgs{
					ManagementGroupId: pulumi.String(config.ClientProjectManagementGroupId),
				},
			},
		})
		if err != nil {
			return err
		}
		// create the next resources in the subscription we just created.
		// TODO: #1 bug here - azure doesn't recognize the subscription we just created with the credentials cached with `az login`
		provider, err := pulumiazurenativesdk.NewProvider(ctx, "new_sub_provider", &pulumiazurenativesdk.ProviderArgs{
			SubscriptionId: sub.Properties.SubscriptionId(),
			TenantId:       pulumi.String(config.TenantId),
			ClientId:       pulumi.String(config.PulumiClientId),
			ClientSecret:   pulumi.String(config.PulumiClientSecret),
		})

		// network settings
		networkRg, err := resources.NewResourceGroup(ctx, "network_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(fmt.Sprintf("rg-%s-network-%s", strings.ToLower(project.Name), env.String())),
			Location:          pulumi.String(config.Region),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}
		vnet, err := network.NewVirtualNetwork(ctx, "network_vnet", &network.VirtualNetworkArgs{
			VirtualNetworkName: pulumi.String(fmt.Sprintf("vnet-%s-%s-%s", strings.ToLower(project.Name), env.String(), strings.ToLower(config.Region))),
			IpAllocations:      network.SubResourceArray{network.SubResourceArgs{Id: pulumi.String(config.ClientDevVnetIpAllocId)}},
			Subnets:            network.SubnetTypeArray{network.SubnetTypeArgs{Name: pulumi.String("default"), IpAllocations: network.SubResourceArray{network.SubResourceArgs{Id: pulumi.String(config.ClientDevVnetIpAllocId)}}}},
			AddressSpace:       network.AddressSpaceArgs{AddressPrefixes: make(pulumi.StringArray, 0)},
			Location:           pulumi.String(config.Region),
		})
		if err != nil {
			return err
		}

		// storage (storage account, azure sql db) //
		storageRg, err := resources.NewResourceGroup(ctx, "storage_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(fmt.Sprintf("rg-%s-storage-%s", strings.ToLower(project.Name), env.String())),
			Location:          pulumi.String(config.Region),
		})
		if err != nil {
			return err
		}
		stAccount, err := storage.NewStorageAccount(ctx, "storage_account_data", &storage.StorageAccountArgs{
			AccountName:           pulumi.String(fmt.Sprintf("st%sdata", strings.ToLower(project.Name))),
			IsHnsEnabled:          pulumi.Bool(true),
			AllowBlobPublicAccess: pulumi.Bool(false), // TODO: confirm network setting on this - need private endpoints
		})
		if err != nil {
			return err
		}

		// security (akv)
		securityRg, err := resources.NewResourceGroup(ctx, "security_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(fmt.Sprintf("rg-%s-security-%s", strings.ToLower(project.Name), env.String())),
			Location:          pulumi.String(config.Region),
		})
		if err != nil {
			return err
		}
		kv, err := keyvault.NewVault(ctx, "keyvault", &keyvault.VaultArgs{
			VaultName:  pulumi.String(fmt.Sprintf("kv-%s-%s", strings.ToLower(project.Name), env.String())),
			Properties: keyvault.VaultPropertiesArgs{EnableRbacAuthorization: pulumi.Bool(false), EnableSoftDelete: pulumi.Bool(true)}, // TODO: network settings
		})

		// analytics (adf, databricks)

		ctx.Export("subscriptionId", sub.Properties.SubscriptionId())
		ctx.Export("networkRgName", networkRg.Name)
		ctx.Export("storageRgName", storageRg.Name)
		ctx.Export("securityRgName", securityRg.Name)
		ctx.Export("storageAccountName", stAccount.Name)
		ctx.Export("vnetName", vnet.Name)
		ctx.Export("akvName", kv.Name)
		return nil
	}
}
