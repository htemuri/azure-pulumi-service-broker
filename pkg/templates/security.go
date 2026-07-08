package templates

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/keyvault/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func (sr *SecurityRequest) newTemplate() (Template, error) {
	s := &Security{
		StackName:        fmt.Sprintf("%s-security", sr.GetDefaultParams().GetEnvironment().shortString()),
		ProviderVersions: []*ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}},
		DependsOn:        []TemplateOptions{TemplateOptions_TEMPLATE_OPTIONS_BASE},
		Request:          sr,
	}
	err := s.validate()
	if err != nil {
		return &Security{}, err
	}
	return s, nil
}

func (s *Security) hash() TemplateOptions {
	return TemplateOptions_TEMPLATE_OPTIONS_SECURITY
}

func (s *Security) getDefaultParams() *DefaultParams {
	return s.GetRequest().GetDefaultParams()
}

func (s *Security) validate() error {
	if s == nil {
		return fmt.Errorf("security can't be nil")
	}
	d, err := getValidDefaultParams(s)
	if err != nil {
		return err
	}
	if d.Region == Region_REGION_UNSPECIFIED {
		d.Region = Region_REGION_EASTUS
	}

	keyvault := s.GetRequest().GetKeyVault()
	if keyvault != nil && keyvault.GetNetworkSettings() != nil {
		pa := keyvault.GetNetworkSettings().GetPrivateEndpoint()
		// if private endpoint settings are specified
		if pa != nil && pa.Enabled == true {
			// auto add vault subresource if not specified
			if len(pa.SubResources) == 0 {
				s.Request.KeyVault.NetworkSettings.PrivateEndpoint.SubResources = []string{"vault"}
			} else {
				// if subresource specified, verify that any arent vault
				for _, r := range pa.SubResources {
					if r != "vault" {
						return fmt.Errorf("keyvault only supports vault subresources")
					}
				}
			}
		}
	}
	return nil
}

func (s *Security) Deploy(ctx context.Context, templateResponses []*TemplatesResponse, autonamingConfig map[string]string, debugOptions optup.Option, streamer optup.Option) (isTemplatesResponse_Response, error) {
	var newResponse TemplatesResponse_Security
	stack, err := createOrSelectStack(ctx, s, autonamingConfig)
	if err != nil {
		return &newResponse, err
	}
	var baseResponse BaseResponse
	for _, t := range templateResponses {
		if t.GetBase() != nil {
			baseResponse = *t.GetBase()
		}
	}
	if baseResponse.GetSubscriptionId() == "" ||
		len(baseResponse.GetSubnets()) == 0 {
		return &newResponse, fmt.Errorf("missing base template response when trying to deploy security template")
	}
	// subnets, ok := cm["subnets"].(SubnetResponse)
	// if !ok {
	// 	return cm, fmt.Errorf("failed getting 'subnets' from cm")
	// }
	stack.SetConfig(ctx, "subscriptionId", auto.ConfigValue{Value: baseResponse.SubscriptionId})
	stack.SetConfig(ctx, "subnetId", auto.ConfigValue{Value: baseResponse.Subnets[0].Id})

	res, err := stack.Up(
		ctx,
		debugOptions,
		streamer)
	if err != nil {
		return &newResponse, fmt.Errorf("failed to update stack: %v\n\n", err)
	}

	resourceGroupName, ok := res.Outputs["resourceGroupName"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get resourceName for security response")
	}
	resourceGroupID, ok := res.Outputs["resourceGroupID"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get resourceGroupID for security response")
	}
	akvURI, ok := res.Outputs["akvURI"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get akvURI for security response")
	}
	akvId, ok := res.Outputs["akvId"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get akvId for security response")
	}
	var privateEndpoints []*PrivateEndpointResponse
	if err := mapstructure.Decode(res.Outputs["privateEndpoints"].Value, &privateEndpoints); err != nil {
		return &newResponse, fmt.Errorf("failed to get privateEndpoints for security response")
	}

	return &TemplatesResponse_Security{
		Security: &SecurityResponse{
			ResourceGroupName: resourceGroupName,
			ResourceGroupId:   resourceGroupID,
			KeyVaultId:        akvId,
			KeyVaultUri:       akvURI,
			PrivateEndpoints:  privateEndpoints,
		},
	}, nil

}

func (s *Security) pulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		subscriptionId := conf.Get("subscriptionId")
		if subscriptionId == "" {
			return fmt.Errorf("failed to get subscriptionId from context config")
		}
		subnetId := conf.Get("subnetId")
		if subnetId == "" {
			return fmt.Errorf("failed to get subnetId from context config")
		}

		var privateEndpoints []pulumi.Input
		defaultParams := s.getDefaultParams()
		projectName := defaultParams.GetProjectName()
		envShort := defaultParams.GetEnvironment().shortString()
		networkSettings := s.GetRequest().GetKeyVault().GetNetworkSettings()

		// creating a new provider that creates all new resources under subscriptionId
		provider, err := pulumiazurenativesdk.NewProvider(ctx, "sub_provider", &pulumiazurenativesdk.ProviderArgs{
			SubscriptionId: pulumi.String(subscriptionId),
			TenantId:       pulumi.String(defaultParams.PulumiProviderCredential.GetTenantId()),
			ClientId:       pulumi.String(defaultParams.PulumiProviderCredential.GetClientId()),
			ClientSecret:   pulumi.String(defaultParams.PulumiProviderCredential.GetClientSecret()),
		})
		if err != nil {
			ctx.Log.Error(err.Error(), nil)
			return err
		}

		resourceGroupName := pulumi.Sprintf("rg-%s-security-%s", strings.ToLower(projectName), strings.ToLower(envShort))
		securityRg, err := resources.NewResourceGroup(ctx, "security_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: resourceGroupName,
			Location:          pulumi.String(s.getDefaultParams().GetRegion().shortString()),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// if keyvault enabled in template
		if s.GetRequest().GetKeyVault().GetDisabled() == false {
			var publicNetworkAccessString string
			if networkSettings.GetPublicNetworkEnabled() {
				publicNetworkAccessString = "enabled"
			} else {
				publicNetworkAccessString = "disabled"
			}

			kv, err := keyvault.NewVault(ctx, fmt.Sprintf("kv-%s-%s", strings.ToLower(projectName), strings.ToLower(envShort)), &keyvault.VaultArgs{
				ResourceGroupName: securityRg.Name,
				Properties: keyvault.VaultPropertiesArgs{
					EnableRbacAuthorization: pulumi.Bool(false),
					EnableSoftDelete:        pulumi.Bool(true),
					TenantId:                pulumi.String(defaultParams.GetPulumiProviderCredential().GetTenantId()),
					Sku: keyvault.SkuArgs{
						Name:   keyvault.SkuNameStandard,
						Family: pulumi.String("A")},
					PublicNetworkAccess: pulumi.String(publicNetworkAccessString)},
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}
			if networkSettings.GetPrivateEndpoint().GetEnabled() {
				// TODO: #2 figure out the private dns zone record thing
				pe, err := network.NewPrivateEndpoint(ctx, "keyvault_pe", &network.PrivateEndpointArgs{
					ResourceGroupName:   securityRg.Name,
					PrivateEndpointName: pulumi.Sprintf("%s-pe", kv.Name),
					Subnet: network.SubnetTypeArgs{
						Id: pulumi.String(subnetId),
					},
					PrivateLinkServiceConnections: network.PrivateLinkServiceConnectionArray{network.PrivateLinkServiceConnectionArgs{
						Name:                 pulumi.String("vault_connection"),
						PrivateLinkServiceId: kv.ID(),
						GroupIds:             pulumi.StringArray{pulumi.String("vault")},
					}},
				}, pulumi.Provider(provider))
				if err != nil {
					return err
				}

				peExport := pulumi.Map(map[string]pulumi.Input{
					"Fqdn":        pe.CustomDnsConfigs.Index(pulumi.Int(0)).Fqdn(),
					"IpAddress":   pe.CustomDnsConfigs.Index(pulumi.Int(0)).IpAddresses().Index(pulumi.Int(0)),
					"DnsZoneName": pulumi.String("privatelink.vaultcore.azure.net"),
				})
				privateEndpoints = append(privateEndpoints, peExport)
			}
			ctx.Export("akvURI", kv.Properties.VaultUri())
			ctx.Export("akvId", kv.ID())
		} else {
			ctx.Export("akvURI", pulumi.String(""))
			ctx.Export("akvId", pulumi.String(""))
		}

		// TODO: i think I can make the private endpoint object better here - like implement the pulumi.Input interface on my custom type
		// private endpoint info export
		var convertedEndpoints []any
		for _, p := range privateEndpoints {
			convertedEndpoints = append(convertedEndpoints, p)
		}
		ctx.Export("privateEndpoints", pulumi.ToArray(convertedEndpoints))
		ctx.Export("resourceGroupName", securityRg.Name)
		ctx.Export("resourceGroupID", securityRg.ID())

		return nil
	}
}
