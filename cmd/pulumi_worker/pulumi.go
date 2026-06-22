package main

import (
	"fmt"
	"strings"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	"github.com/pulumi/pulumi-azure-native-sdk/datafactory/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/keyvault/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/subscription/v3"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk/v3"
	azad "github.com/pulumi/pulumi-azuread/sdk/v6/go/azuread"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type PulumiJobs struct {
	env     Environment
	project *template.Project
	config  Config
}

func (pj *PulumiJobs) genResourceGroupName(topic string) pulumi.StringOutput {
	return pulumi.Sprintf("rg-%s-%s-%s", strings.ToLower(pj.project.Name), topic, pj.env.String())
}

func (pj *PulumiJobs) ResourceJob() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {

		var privateEndpoints []pulumi.Input
		subscriptionName := pulumi.Sprintf("[%s] Client Project: %s", strings.ToUpper(pj.env.String()), strings.ToUpper(pj.project.Name))

		sub, err := subscription.NewAlias(ctx, "subscription", &subscription.AliasArgs{
			Properties: subscription.PutAliasRequestPropertiesArgs{
				DisplayName:  subscriptionName,
				BillingScope: pulumi.String(pj.config.BillingScope),
				Workload:     pulumi.String("Production"),
				AdditionalProperties: subscription.PutAliasRequestAdditionalPropertiesArgs{
					ManagementGroupId: pulumi.String(pj.config.ClientProjectManagementGroupId),
				},
			},
		})
		if err != nil {
			return err
		}
		// create the next resources in the subscription we just created.
		// TODO: #1 bug here - azure doesn't recognize the subscription we just created with the credentials cached with `az login`
		projectProvider, err := pulumiazurenativesdk.NewProvider(ctx, "new_sub_provider", &pulumiazurenativesdk.ProviderArgs{
			SubscriptionId: sub.Properties.SubscriptionId(),
			TenantId:       pulumi.String(pj.config.TenantId),
			ClientId:       pulumi.String(pj.config.PulumiClientId),
			ClientSecret:   pulumi.String(pj.config.PulumiClientSecret),
		})

		// network settings
		networkRg, err := resources.NewResourceGroup(ctx, "network_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pj.genResourceGroupName("network"),
			Location:          pulumi.String(pj.config.Region),
		}, pulumi.Provider(projectProvider))
		if err != nil {
			return err
		}

		vnet, err := network.NewVirtualNetwork(ctx, fmt.Sprintf("vnet-%s-%s-%s", strings.ToLower(pj.project.Name), pj.env.String(), strings.ToLower(pj.config.Region)), &network.VirtualNetworkArgs{
			ResourceGroupName: networkRg.Name,
			Subnets:           network.SubnetTypeArray{network.SubnetTypeArgs{Name: pulumi.String("default"), IpamPoolPrefixAllocations: network.IpamPoolPrefixAllocationArray{network.IpamPoolPrefixAllocationArgs{Id: pulumi.String(pj.config.ClientDevVnetIpAllocId), NumberOfIpAddresses: pulumi.String("32")}}}},
			AddressSpace:      network.AddressSpaceArgs{AddressPrefixes: make(pulumi.StringArray, 0), IpamPoolPrefixAllocations: network.IpamPoolPrefixAllocationArray{network.IpamPoolPrefixAllocationArgs{Id: pulumi.String(pj.config.ClientDevVnetIpAllocId), NumberOfIpAddresses: pulumi.String("32")}}},
			Location:          pulumi.String(pj.config.Region),
		}, pulumi.Provider(projectProvider))
		if err != nil {
			return err
		}

		defaultSubnetId := vnet.Subnets.Index(pulumi.Int(0)).Id() // grab first subnet in vnet output - should be the 'default' subnet

		// storage (storage account, azure sql db)
		storageRg, err := resources.NewResourceGroup(ctx, "storage_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pj.genResourceGroupName("storage"),
			Location:          pulumi.String(pj.config.Region),
		}, pulumi.Provider(projectProvider))
		if err != nil {
			return err
		}

		if pj.project.StorageAccount.Enabled {
			stAccount, err := storage.NewStorageAccount(ctx, fmt.Sprintf("st%sdata", strings.ToLower(pj.project.Name)), &storage.StorageAccountArgs{
				ResourceGroupName:     storageRg.Name,
				IsHnsEnabled:          pulumi.Bool(true),
				AllowBlobPublicAccess: pulumi.Bool(false), // TODO: confirm network setting on this - need private endpoints
				PublicNetworkAccess:   pulumi.String("disabled"),
				Kind:                  pulumi.String("StorageV2"),
				Sku:                   storage.SkuArgs{Name: pulumi.String("Standard_LRS")},
			}, pulumi.Provider(projectProvider))
			if err != nil {
				return err
			}

			// create the private endpoints for the storage account
			for _, sub := range pj.project.StorageAccount.SubResources {
				sub := sub.ShortString() // convert the enum to the short form string
				// TODO: #2 figure out the private dns zone record thing
				pe, err := network.NewPrivateEndpoint(ctx, fmt.Sprintf("storage_account_%s_pe", sub), &network.PrivateEndpointArgs{
					ResourceGroupName:   storageRg.Name,
					PrivateEndpointName: pulumi.Sprintf("%s-%s-pe", stAccount.Name, sub),
					Subnet: network.SubnetTypeArgs{
						Id: defaultSubnetId,
					},
					PrivateLinkServiceConnections: network.PrivateLinkServiceConnectionArray{network.PrivateLinkServiceConnectionArgs{
						Name:                 pulumi.Sprintf("%s_connection", sub),
						PrivateLinkServiceId: stAccount.ID(),
						GroupIds:             pulumi.StringArray{pulumi.String(sub)},
					}},
				}, pulumi.Provider(projectProvider))
				if err != nil {
					return err
				}
				peExport := pulumi.Map(map[string]pulumi.Input{ // exporting this to setup dns zone records in another service
					"Fqdn":        pe.CustomDnsConfigs.Index(pulumi.Int(0)).Fqdn(),
					"IpAddress":   pe.CustomDnsConfigs.Index(pulumi.Int(0)).IpAddresses().Index(pulumi.Int(0)),
					"DnsZoneName": pulumi.Sprintf("privatelink.%s.core.windows.net", sub),
				},
				)
				privateEndpoints = append(privateEndpoints, peExport)
			}

			ctx.Export("storageAccountName", stAccount.Name)
		}

		// security (akv)
		securityRg, err := resources.NewResourceGroup(ctx, "security_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pj.genResourceGroupName("security"),
			Location:          pulumi.String(pj.config.Region),
		}, pulumi.Provider(projectProvider))
		if err != nil {
			return err
		}
		if pj.project.KeyVaultOptions.Enabled {
			kv, err := keyvault.NewVault(ctx, fmt.Sprintf("kv-%s-%s", strings.ToLower(pj.project.Name), pj.env.String()), &keyvault.VaultArgs{
				ResourceGroupName: securityRg.Name,
				Properties:        keyvault.VaultPropertiesArgs{EnableRbacAuthorization: pulumi.Bool(false), EnableSoftDelete: pulumi.Bool(true), TenantId: pulumi.String(pj.config.TenantId), Sku: keyvault.SkuArgs{Name: keyvault.SkuNameStandard, Family: pulumi.String("A")}, PublicNetworkAccess: pulumi.String("disabled")},
			}, pulumi.Provider(projectProvider))
			if err != nil {
				return err
			}
			// TODO: #2 figure out the private dns zone record thing
			pe, err := network.NewPrivateEndpoint(ctx, "keyvault_pe", &network.PrivateEndpointArgs{
				ResourceGroupName:   securityRg.Name,
				PrivateEndpointName: pulumi.Sprintf("%s-pe", kv.Name),
				Subnet: network.SubnetTypeArgs{
					Id: defaultSubnetId,
				},
				PrivateLinkServiceConnections: network.PrivateLinkServiceConnectionArray{network.PrivateLinkServiceConnectionArgs{
					Name:                 pulumi.String("vault_connection"),
					PrivateLinkServiceId: kv.ID(),
					GroupIds:             pulumi.StringArray{pulumi.String("vault")},
				}},
			}, pulumi.Provider(projectProvider))

			peExport := pulumi.Map(map[string]pulumi.Input{
				"Fqdn":        pe.CustomDnsConfigs.Index(pulumi.Int(0)).Fqdn(),
				"IpAddress":   pe.CustomDnsConfigs.Index(pulumi.Int(0)).IpAddresses().Index(pulumi.Int(0)),
				"DnsZoneName": pulumi.String("privatelink.vaultcore.azure.net"),
			})
			privateEndpoints = append(privateEndpoints, peExport)

			ctx.Export("akvName", kv.Name)
		}

		// analytics (adf, databricks)

		analyticsRg, err := resources.NewResourceGroup(ctx, "analytics_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: pj.genResourceGroupName("analytics"),
			Location:          pulumi.String(pj.config.Region),
		}, pulumi.Provider(projectProvider))
		if err != nil {
			return err
		}
		if pj.project.DataFactoryOptions.Enabled {
			df, err := datafactory.NewFactory(ctx, fmt.Sprintf("adf-%s-%s", strings.ToLower(pj.project.Name), pj.env.String()), &datafactory.FactoryArgs{
				ResourceGroupName:   analyticsRg.Name,
				PublicNetworkAccess: pulumi.String("enabled"), // dont need to restrict access to the adf via pe
			}, pulumi.Provider(projectProvider))
			if err != nil {
				return err
			}
			ctx.Export("adfName", df.Name)
		}

		// TODO: i think I can make the private endpoint object better here - like implement the pulumi.Input interface on my custom type
		// private endpoint info export
		var convertedEndpoints []any
		for _, p := range privateEndpoints {
			convertedEndpoints = append(convertedEndpoints, p)
		}
		ctx.Export("privateEndpoints", pulumi.ToArray(convertedEndpoints))

		ctx.Export("subscriptionId", sub.Properties.SubscriptionId())
		ctx.Export("networkRgName", networkRg.Name)
		ctx.Export("storageRgName", storageRg.Name)
		ctx.Export("securityRgName", securityRg.Name)
		ctx.Export("vnetName", vnet.Name)

		return nil
	}
}

func (pj *PulumiJobs) EntraJob() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		provider, err := azad.NewProvider(ctx, "entra_provider", &azad.ProviderArgs{
			TenantId:     pulumi.String(pj.config.TenantId),
			ClientId:     pulumi.String(pj.config.PulumiClientId),
			ClientSecret: pulumi.String(pj.config.PulumiClientSecret),
		})
		if err != nil {
			return err
		}
		// entra objects - eventually I want to build a better go client or pulumi provider for azure graph. the official microsoft graph go sdk sucks and this azuread provider is just a wrapper on a terraform provider
		for k := range template.RoleType_name {
			if k == 0 { // skip unspecified role type
				continue
			}
			role := template.RoleType(k).ShortString()
			users := pj.project.RoleUserList(template.RoleType(k))
			var user_ids pulumi.StringArray
			for _, user := range users {
				user_ids = append(user_ids, pulumi.String(user.ObjectId))
			}
			g, err := azad.NewGroup(ctx, fmt.Sprintf("%s_group", role), &azad.GroupArgs{
				DisplayName:     pulumi.Sprintf("grp_%s_%s", strings.ToLower(pj.project.Name), role),
				Description:     pulumi.Sprintf("%s persona group for the %s project in the %s environment", cases.Title(language.English).String(role), cases.Title(language.English).String(pj.project.Name)),
				Owners:          append(user_ids, pulumi.ToStringArray(pj.config.EntraIdAdminObjectIds)...), // allowing role type to own its group (along with entra admins). can change to fit your needs
				Members:         user_ids,
				MailEnabled:     pulumi.Bool(false),
				SecurityEnabled: pulumi.Bool(true),
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}
			ctx.Export(fmt.Sprintf("%sGroupName", role), g.DisplayName)
		}
		return nil
	}
}
