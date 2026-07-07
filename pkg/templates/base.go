package templates

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/network/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/subscription/v3"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk/v3"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (br *BaseRequest) newTemplate() (Template, error) {
	b := &Base{
		StackName:        fmt.Sprintf("%s-base", br.GetDefaultParams().GetEnvironment().ShortString()),
		ProviderVersions: []*ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}},
		DependsOn:        []TemplateOptions{},
		Request:          br,
	}
	err := b.validate()
	if err != nil {
		return &Base{}, err
	}
	return b, nil
}

func (b *Base) hash() TemplateOptions {
	return TemplateOptions_TEMPLATE_OPTIONS_BASE
}

func (b *Base) getDefaultParams() *DefaultParams {
	return b.GetRequest().GetDefaultParams()
}

func (b *Base) validate() error {
	br := b.GetRequest()
	if b == nil {
		return fmt.Errorf("base can't be nil")
	}
	d, err := getValidDefaultParams(b)
	if err != nil {
		return err
	}
	if d.Region == Region_REGION_UNSPECIFIED {
		d.Region = Region_REGION_EASTUS
	}
	s := br.GetSubscription()
	n := br.GetVirtualNetwork()
	if s == nil {
		return fmt.Errorf("subscription args can't be nil")
	}
	if n == nil {
		return fmt.Errorf("network args can't be nil")
	}

	if s.GetSubscriptionId() == "" {
		if s.GetBillingScope() == "" {
			return fmt.Errorf("billing scope must be provided if not using existing subscription")
		}
		if s.GetManagementGroupId() == "" {
			return fmt.Errorf("management group id must be provided if not using existing subscription")
		}
	}

	if n.GetIpamPoolPrefixAllocations().GetIpamPoolResourceId() == "" {
		return fmt.Errorf("missing resource id of ipam pool for vnet")
	}
	if n.GetIpamPoolPrefixAllocations().GetNumberOfIpAddresses() < 32 {
		return fmt.Errorf("number of IP addresses for vnet should be above 32")
	}
	totalSubnetIpsUsed := int32(0)
	for _, subnet := range n.GetSubnets() {
		if subnet.GetName() == "" {
			return fmt.Errorf("missing name from subnet args")
		}
		if subnet.GetNumberOfIpAddresses() < 32 {
			return fmt.Errorf("number of IP addresses for subnets must be above 32")
		} else if subnet.GetNumberOfIpAddresses() > (n.GetIpamPoolPrefixAllocations().GetNumberOfIpAddresses() - totalSubnetIpsUsed) {
			return fmt.Errorf("not enough available IPs for subnet %s in the VNET with the previously allocated subnets", subnet.GetName())
		}
		totalSubnetIpsUsed += subnet.GetNumberOfIpAddresses()
	}
	return nil
}

func (b *Base) Deploy(ctx context.Context, templateResponses []*TemplatesResponse, autonamingConfig map[string]string, debugOptions optup.Option, streamer optup.Option) (isTemplatesResponse_Response, error) {
	var newResponse TemplatesResponse_Base
	s, err := createOrSelectStack(ctx, b, autonamingConfig)
	if err != nil {
		return &newResponse, err
	}
	res, err := s.Up(
		ctx,
		debugOptions,
		streamer)
	if err != nil {
		return &newResponse, fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	subscriptionId, ok := res.Outputs["subscriptionId"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get subscriptionId for base response: %s\n", err)
	}
	vnetId, ok := res.Outputs["vnetId"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get vnetId for base response: %s\n", err)
	}
	// subnets, ok := res.Outputs["subnets"].Value.([]string)
	// if !ok {
	// 	return cm, fmt.Errorf("failed to get subnets for base response: %s\n", err)
	// }

	return &TemplatesResponse_Base{
		Base: &BaseResponse{
			SubscriptionId: subscriptionId,
			VnetId:         vnetId,
		},
	}, nil
}

func (b *Base) pulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		defaultParams := b.getDefaultParams()
		subscriptionArgs := b.GetRequest().GetSubscription()
		virtualNetworkArgs := b.GetRequest().GetVirtualNetwork()
		projectName := defaultParams.GetProjectName()
		envShort := defaultParams.GetEnvironment().ShortString()
		ctx.Log.Info("pre provider", &pulumi.LogArgs{})

		var provider *pulumiazurenativesdk.Provider

		var subscriptionId pulumi.StringPtrInput
		if subscriptionArgs.GetSubscriptionId() == "" {
			provider, err := pulumiazurenativesdk.NewProvider(ctx, "stale_sub_provider", &pulumiazurenativesdk.ProviderArgs{
				TenantId:     pulumi.String(defaultParams.PulumiProviderCredential.GetTenantId()),
				ClientId:     pulumi.String(defaultParams.PulumiProviderCredential.GetClientId()),
				ClientSecret: pulumi.String(defaultParams.PulumiProviderCredential.GetClientSecret()),
			})
			if err != nil {
				ctx.Log.Error(err.Error(), nil)
				return err
			}

			subscriptionName := pulumi.Sprintf("[%s] Project: %s", strings.ToUpper(envShort), strings.ToUpper(projectName))

			sub, err := subscription.NewAlias(ctx, "subscription", &subscription.AliasArgs{
				Properties: subscription.PutAliasRequestPropertiesArgs{
					DisplayName:  subscriptionName,
					BillingScope: pulumi.String(subscriptionArgs.BillingScope),
					Workload:     pulumi.String("Production"),
					AdditionalProperties: subscription.PutAliasRequestAdditionalPropertiesArgs{
						ManagementGroupId: pulumi.String(subscriptionArgs.ManagementGroupId),
					},
				},
			}, pulumi.Provider(provider))
			if err != nil {
				ctx.Log.Error(err.Error(), nil)

				return err
			}
			subscriptionId = sub.Properties.SubscriptionId()
		} else {
			subscriptionId = pulumi.String(subscriptionArgs.GetSubscriptionId())
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
			TenantId:       pulumi.String(defaultParams.PulumiProviderCredential.GetTenantId()),
			ClientId:       pulumi.String(defaultParams.PulumiProviderCredential.GetClientId()),
			ClientSecret:   pulumi.String(defaultParams.PulumiProviderCredential.GetClientSecret()),
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
			Location:          pulumi.String(defaultParams.Region.ShortString()),
		}, pulumi.Provider(provider))
		if err != nil {
			ctx.Log.Error(err.Error(), nil)

			return err
		}

		ipamPool := virtualNetworkArgs.GetIpamPoolPrefixAllocations()
		vnetName := fmt.Sprintf("vnet-%s-%s-%s", strings.ToLower(projectName), strings.ToLower(envShort), strings.ToLower(defaultParams.Region.ShortString()))

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
			Location: pulumi.String(defaultParams.Region.ShortString()),
		}, pulumi.Provider(provider))
		if err != nil {
			ctx.Log.Error(err.Error(), nil)
			return err
		}

		var subnets []*SubnetArgs
		if len(virtualNetworkArgs.GetSubnets()) > 0 {
			subnets = append(subnets, virtualNetworkArgs.GetSubnets()...)
		} else {
			// default subnet settings if not specified in base struct
			subnets = append(subnets, &SubnetArgs{
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
						Id:                  pulumi.String(ipamPool.GetIpamPoolResourceId()),
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
