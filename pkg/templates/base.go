package templates

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/network/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/subscription/v3"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk/v3"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func NewBaseTemplate(projectName string, environment Environment, region Region, subscriptionArgs *SubscriptionArgs, networkArgs *NetworkArgs, cred *PulumiProviderCredentialArgs) (*Base, error) {

	if projectName == "" {
		return &Base{}, fmt.Errorf("projectName cannot be an empty string")
	}
	if environment == Environment_ENVIRONMENT_UNSPECIFIED {
		return &Base{}, fmt.Errorf("environment must be specified")
	}
	if region == Region_REGION_UNSPECIFIED {
		region = Region_REGION_EASTUS
	}

	if subscriptionArgs.GetSubscriptionId() == "" {
		if subscriptionArgs.BillingScope == "" {
			return &Base{}, fmt.Errorf("billing scope must be provided if not using existing subscription")
		}
		if subscriptionArgs.ManagementGroupId == "" {
			return &Base{}, fmt.Errorf("management group id must be provided if not using existing subscription")
		}
	}

	if networkArgs.IpamPoolPrefixAllocations.IpamPoolResourceId == "" {
		return &Base{}, fmt.Errorf("missing resource id of ipam pool for vnet")
	}
	if networkArgs.IpamPoolPrefixAllocations.NumberOfIpAddresses < 32 {
		return &Base{}, fmt.Errorf("number of IP addresses for vnet should be above 32")
	}
	totalSubnetIpsUsed := int32(0)
	for _, subnet := range networkArgs.Subnets {
		if subnet.Name == "" {
			return &Base{}, fmt.Errorf("missing name from subnet args")
		}
		if subnet.NumberOfIpAddresses < 32 {
			return &Base{}, fmt.Errorf("number of IP addresses for subnets must be above 32")
		} else if subnet.NumberOfIpAddresses > (networkArgs.IpamPoolPrefixAllocations.NumberOfIpAddresses - totalSubnetIpsUsed) {
			return &Base{}, fmt.Errorf("not enough available IPs for subnet %s in the VNET with the previously allocated subnets", subnet.Name)
		}
		totalSubnetIpsUsed += subnet.NumberOfIpAddresses
	}

	if _, err := cred.Validate(); err != nil {
		return &Base{}, err
	}

	stackName := fmt.Sprintf("%s-base", environment.ShortString())
	providers := []*ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}}

	base := Base{
		DefaultParams: &DefaultParams{
			Enabled:     true,
			ProjectName: projectName,
			StackName:   stackName,
			Environment: environment,
			Region:      region,
			Providers:   providers,
			DependsOn:   []TemplateOptions{},
		},
		Subscription:   subscriptionArgs,
		VirtualNetwork: networkArgs,
	}
	return &base, nil
}

func (b *Base) PulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		projectName := b.DefaultParams.GetProjectName()
		envShort := b.DefaultParams.Environment.ShortString()
		ctx.Log.Info("pre provider", &pulumi.LogArgs{})

		var provider *pulumiazurenativesdk.Provider

		var subscriptionId pulumi.StringPtrInput
		if b.Subscription.GetSubscriptionId() == "" {
			provider, err := pulumiazurenativesdk.NewProvider(ctx, "stale_sub_provider", &pulumiazurenativesdk.ProviderArgs{
				TenantId:     pulumi.String(b.DefaultParams.PulumiProviderCredential.GetTenantId()),
				ClientId:     pulumi.String(b.DefaultParams.PulumiProviderCredential.GetClientId()),
				ClientSecret: pulumi.String(b.DefaultParams.PulumiProviderCredential.GetClientSecret()),
			})
			if err != nil {
				ctx.Log.Error(err.Error(), nil)
				return err
			}

			subscriptionName := pulumi.Sprintf("[%s] Project: %s", strings.ToUpper(envShort), strings.ToUpper(projectName))

			sub, err := subscription.NewAlias(ctx, "subscription", &subscription.AliasArgs{
				Properties: subscription.PutAliasRequestPropertiesArgs{
					DisplayName:  subscriptionName,
					BillingScope: pulumi.String(b.Subscription.BillingScope),
					Workload:     pulumi.String("Production"),
					AdditionalProperties: subscription.PutAliasRequestAdditionalPropertiesArgs{
						ManagementGroupId: pulumi.String(b.Subscription.ManagementGroupId),
					},
				},
			}, pulumi.Provider(provider))
			if err != nil {
				ctx.Log.Error(err.Error(), nil)

				return err
			}
			subscriptionId = sub.Properties.SubscriptionId()
		} else {
			subscriptionId = pulumi.String(b.Subscription.GetSubscriptionId())
		}

		// adding this because of stale azure login cache after creating subscription
		refreshCmd, err := local.NewCommand(ctx, "refresh_cred", &local.CommandArgs{
			Create: pulumi.String("az account list --refresh"),
		})
		if err != nil {
			return err
		}

		// creating a new provider that creates all new resources under subscriptionId
		provider, err = pulumiazurenativesdk.NewProvider(ctx, "sub_provider", &pulumiazurenativesdk.ProviderArgs{
			SubscriptionId: subscriptionId,
			TenantId:       pulumi.String(b.DefaultParams.PulumiProviderCredential.GetTenantId()),
			ClientId:       pulumi.String(b.DefaultParams.PulumiProviderCredential.GetClientId()),
			ClientSecret:   pulumi.String(b.DefaultParams.PulumiProviderCredential.GetClientSecret()),
		}, pulumi.DependsOn([]pulumi.Resource{refreshCmd}))
		if err != nil {
			ctx.Log.Error(err.Error(), nil)
			return err
		}

		ctx.Log.Info("past provider", nil)

		// network settings
		resourceGroupName := pulumi.Sprintf("rg-%s-network-%s", strings.ToLower(projectName), strings.ToLower(envShort))
		networkRg, err := resources.NewResourceGroup(ctx, "network_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: resourceGroupName,
			Location:          pulumi.String(b.DefaultParams.Region.ShortString()),
		}, pulumi.Provider(provider))
		if err != nil {
			ctx.Log.Error(err.Error(), nil)

			return err
		}

		ipamPool := b.VirtualNetwork.IpamPoolPrefixAllocations
		vnetName := fmt.Sprintf("vnet-%s-%s-%s", strings.ToLower(projectName), strings.ToLower(envShort), strings.ToLower(b.DefaultParams.Region.ShortString()))

		var subnets network.SubnetTypeArray
		if len(b.VirtualNetwork.Subnets) > 0 {
			for _, subnet := range b.VirtualNetwork.Subnets {
				subnets = append(subnets, network.SubnetTypeArgs{
					Name: pulumi.String(subnet.Name),
					IpamPoolPrefixAllocations: network.IpamPoolPrefixAllocationArray{
						network.IpamPoolPrefixAllocationArgs{
							Id:                  pulumi.String(ipamPool.IpamPoolResourceId),
							NumberOfIpAddresses: pulumi.String(strconv.Itoa(int(subnet.GetNumberOfIpAddresses()))),
						},
					},
				})
			}
		} else {
			// default subnet settings if not specified in base struct
			subnets = append(subnets, network.SubnetTypeArgs{
				Name: pulumi.String("default"),
				IpamPoolPrefixAllocations: network.IpamPoolPrefixAllocationArray{
					network.IpamPoolPrefixAllocationArgs{
						Id:                  pulumi.String(ipamPool.IpamPoolResourceId),
						NumberOfIpAddresses: pulumi.String("32"),
					},
				},
			})
		}
		vnet, err := network.NewVirtualNetwork(ctx, vnetName, &network.VirtualNetworkArgs{
			ResourceGroupName: networkRg.Name,
			Subnets:           subnets,
			AddressSpace: network.AddressSpaceArgs{
				AddressPrefixes: make(pulumi.StringArray, 0),
				IpamPoolPrefixAllocations: network.IpamPoolPrefixAllocationArray{
					network.IpamPoolPrefixAllocationArgs{
						Id:                  pulumi.String(ipamPool.IpamPoolResourceId),
						NumberOfIpAddresses: pulumi.String(strconv.Itoa(int(ipamPool.GetNumberOfIpAddresses()))),
					},
				},
			},
			Location: pulumi.String(b.DefaultParams.Region.ShortString()),
		}, pulumi.Provider(provider))
		if err != nil {
			ctx.Log.Error(err.Error(), nil)

			return err
		}
		ctx.Export("subscriptionId", subscriptionId)
		ctx.Export("vnet", vnet)
		ctx.Export("provider", provider)
		return nil
	}
}
