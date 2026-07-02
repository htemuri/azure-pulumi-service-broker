package templates

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	templates "github.com/htemuri/azure-pulumi-service-broker/gen/go/templates/v1"

	"github.com/pulumi/pulumi-azure-native-sdk/network/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/subscription/v3"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk/v3"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func NewBaseTemplate(projectName string, environment templates.Environment, region templates.Region, subscriptionArgs *templates.SubscriptionArgs, networkArgs *templates.NetworkArgs, cred *templates.PulumiProviderCredentialArgs) (*templates.Base, error) {
	b := templates.Base{
		DefaultParams: &templates.DefaultParams{
			ProjectName:              projectName,
			Environment:              environment,
			Region:                   region,
			PulumiProviderCredential: cred,
		},
		Subscription:   subscriptionArgs,
		VirtualNetwork: networkArgs,
	}
	err := b.Validate()
	if err != nil {
		return &templates.Base{}, err
	}
	return &b, nil
}

func (b *Base) Hash() templates.TemplateOptions {
	return templates.TemplateOptions_TEMPLATE_OPTIONS_BASE
}

func (b *Base) GetProjectName() string {
	return b.DefaultParams.GetProjectName()
}

func (b *Base) GetStackName() string {
	return fmt.Sprintf("%s-base", b.GetDefaultParams().Environment.ShortString())
}

func (b *Base) GetProviders() []*templates.ProviderVersion {
	return []*templates.ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}}
}

func (b *Base) GetDependsOn() []templates.TemplateOptions {
	return []templates.TemplateOptions{}
}

func (b *Base) Validate() error {
	if b == nil {
		return fmt.Errorf("base can't be nil")
	}
	d, err := GetValidDefaultParams(b)
	if err != nil {
		return err
	}
	if d.Region == templates.Region_REGION_UNSPECIFIED {
		d.Region = templates.Region_REGION_EASTUS
	}
	s := b.GetSubscription()
	n := b.GetVirtualNetwork()
	if s == nil {
		return fmt.Errorf("subscription args can't be nil")
	}
	if n == nil {
		return fmt.Errorf("network args can't be nil")
	}

	if b.Subscription.GetSubscriptionId() == "" {
		if b.Subscription.BillingScope == "" {
			return fmt.Errorf("billing scope must be provided if not using existing subscription")
		}
		if b.Subscription.ManagementGroupId == "" {
			return fmt.Errorf("management group id must be provided if not using existing subscription")
		}
	}

	if n.IpamPoolPrefixAllocations.IpamPoolResourceId == "" {
		return fmt.Errorf("missing resource id of ipam pool for vnet")
	}
	if n.IpamPoolPrefixAllocations.NumberOfIpAddresses < 32 {
		return fmt.Errorf("number of IP addresses for vnet should be above 32")
	}
	totalSubnetIpsUsed := int32(0)
	for _, subnet := range n.Subnets {
		if subnet.Name == "" {
			return fmt.Errorf("missing name from subnet args")
		}
		if subnet.NumberOfIpAddresses < 32 {
			return fmt.Errorf("number of IP addresses for subnets must be above 32")
		} else if subnet.NumberOfIpAddresses > (n.IpamPoolPrefixAllocations.NumberOfIpAddresses - totalSubnetIpsUsed) {
			return fmt.Errorf("not enough available IPs for subnet %s in the VNET with the previously allocated subnets", subnet.Name)
		}
		totalSubnetIpsUsed += subnet.NumberOfIpAddresses
	}
	return nil
}

func (b *Base) Deploy(ctx context.Context, cm map[string]any, autonamingConfig map[string]string) (map[string]any, error) {
	s, err := createOrSelectStack(b, ctx, autonamingConfig)
	if err != nil {
		return cm, err
	}
	// // 1 - 11 (least verbose to most verbose)
	// logLevel := uint(2)

	// debugLogging := optup.DebugLogging(debug.LoggingOptions{
	// 	LogToStdErr:   true,
	// 	LogLevel:      &logLevel,
	// 	FlowToPlugins: true,
	// 	Debug:         true,
	// })

	streamer := optup.ProgressStreams(os.Stdout)
	res, err := s.Up(
		ctx,
		// debugLogging,
		streamer)
	if err != nil {
		return cm, fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	cm["subscriptionId"] = res.Outputs["subscriptionId"].Value
	cm["vnetId"] = res.Outputs["vnetId"].Value
	cm["subnets"] = res.Outputs["subnets"].Value
	return cm, nil
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

		vnet, err := network.NewVirtualNetwork(ctx, vnetName, &network.VirtualNetworkArgs{
			ResourceGroupName: networkRg.Name,
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

		var subnets []*templates.SubnetArgs
		if len(b.VirtualNetwork.Subnets) > 0 {
			subnets = append(subnets, b.VirtualNetwork.GetSubnets()...)
		} else {
			// default subnet settings if not specified in base struct
			subnets = append(subnets, &templates.SubnetArgs{
				Name:                "default",
				NumberOfIpAddresses: 32,
			})
		}
		for _, s := range subnets {
			_, err := network.NewSubnet(ctx, fmt.Sprintf("subnet-%s", s.GetName()), &network.SubnetArgs{
				Name:               pulumi.String(s.GetName()),
				VirtualNetworkName: vnet.Name,
				ResourceGroupName:  networkRg.Name,
				IpamPoolPrefixAllocations: network.IpamPoolPrefixAllocationArray{
					network.IpamPoolPrefixAllocationArgs{
						Id:                  pulumi.String(ipamPool.IpamPoolResourceId),
						NumberOfIpAddresses: pulumi.String(strconv.Itoa(int(s.GetNumberOfIpAddresses()))),
					},
				},
			}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{vnet}))
			if err != nil {
				return err
			}
		}

		ctx.Export("subscriptionId", subscriptionId)
		ctx.Export("vnetId", vnet.ID())
		ctx.Export("subnets", vnet.Subnets)
		return nil
	}
}
