package templates

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v3"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func (sr *StorageRequest) newTemplate() (Template, error) {
	s := &Storage{
		StackName:        fmt.Sprintf("%s-storage", sr.GetDefaultParams().GetEnvironment().shortString()),
		ProviderVersions: []*ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}},
		DependsOn:        []TemplateOptions{TemplateOptions_TEMPLATE_OPTIONS_BASE},
		Request:          sr,
	}
	err := s.validate()
	if err != nil {
		return &Storage{}, err
	}
	return s, nil
}

func (s *Storage) hash() TemplateOptions {
	return TemplateOptions_TEMPLATE_OPTIONS_STORAGE
}

func (s *Storage) getDefaultParams() *DefaultParams {
	return s.GetRequest().GetDefaultParams()
}

func (s *Storage) validate() error {
	if s == nil {
		return fmt.Errorf("Storage can't be nil")
	}
	d, err := getValidDefaultParams(s)
	if err != nil {
		return err
	}
	if d.Region == Region_REGION_UNSPECIFIED {
		d.Region = Region_REGION_EASTUS
	}

	storage := s.GetRequest().GetStorageAccount()

	// validate storage config
	if storage != nil && storage.GetDisabled() != true {
		if storage.Kind == StorAcctKind_STOR_ACCT_KIND_UNSPECIFIED {
			storage.Kind = StorAcctKind_STOR_ACCT_KIND_STORAGE_V2
		}
		if storage.Sku == StorAcctSKU_STOR_ACCT_SKU_UNSPECIFIED {
			storage.Sku = StorAcctSKU_STOR_ACCT_SKU_STANDARD_LRS
		}
		kind := storage.GetKind()
		sku := storage.GetSku()

		if (sku == StorAcctSKU_STOR_ACCT_SKU_PREMIUMV2_LRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_PREMIUMV2_ZRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_LRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_ZRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_GRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_GZRS) && kind != StorAcctKind_STOR_ACCT_KIND_FILE_STORAGE {
			return fmt.Errorf("sku %s only supported with %s storage account kind", sku.shortString(), StorAcctKind_STOR_ACCT_KIND_FILE_STORAGE.shortString())
		}

		if (sku == StorAcctSKU_STOR_ACCT_SKU_STANDARD_LRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARD_ZRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARD_GRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARD_GZRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARD_RAGRS ||
			sku == StorAcctSKU_STOR_ACCT_SKU_STANDARD_RAGZRS) && kind != StorAcctKind_STOR_ACCT_KIND_STORAGE_V2 {
			return fmt.Errorf("sku %s only supported with %s storage account kind", sku.shortString(), StorAcctKind_STOR_ACCT_KIND_STORAGE_V2.shortString())
		}
	}

	// validate network config
	if storage != nil && storage.GetNetworkSettings() != nil && storage.GetDisabled() != true {
		pa := storage.GetNetworkSettings().GetPrivateEndpoint()
		// if private endpoint settings are specified
		if pa != nil && pa.Enabled == true {
			if len(pa.SubResources) == 0 {
				// auto add blob and dfs subresources if not specified
				s.Request.StorageAccount.NetworkSettings.PrivateEndpoint.SubResources = []string{"blob", "dfs"}
			} else {
				for _, r := range pa.SubResources {
					if r != "blob" &&
						r != "dfs" &&
						r != "queue" &&
						r != "file" &&
						r != "web" {
						return fmt.Errorf("storage accounts only support the following subresources: 'blob', 'dfs', 'queue', 'file', and 'web'")
					}
				}
			}
		}
	}

	return nil
}

func (s *Storage) Deploy(ctx context.Context, templateResponses []*TemplatesResponse, autonamingConfig map[string]string, debugOptions optup.Option, streamer optup.Option) (isTemplatesResponse_Response, error) {
	var newResponse TemplatesResponse_Storage
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
		return &newResponse, fmt.Errorf("missing base template response when trying to deploy storage template")
	}

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
	storageAccountName, ok := res.Outputs["storageAccountName"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get storageAccountName for security response")
	}
	storageAccountId, ok := res.Outputs["storageAccountId"].Value.(string)
	if !ok {
		return &newResponse, fmt.Errorf("failed to get storageAccountId for security response")
	}
	var privateEndpoints []*PrivateEndpointResponse
	if err := mapstructure.Decode(res.Outputs["privateEndpoints"].Value, &privateEndpoints); err != nil {
		return &newResponse, fmt.Errorf("failed to get privateEndpoints for security response")
	}

	return &TemplatesResponse_Storage{
		Storage: &StorageResponse{
			ResourceGroupName:  resourceGroupName,
			ResourceGroupId:    resourceGroupID,
			StorageAccountName: storageAccountName,
			StorageAccountId:   storageAccountId,
			PrivateEndpoints:   privateEndpoints,
		},
	}, nil
}

func (s *Storage) pulumiRunFunc() pulumi.RunFunc {
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
		storageSettings := s.GetRequest().GetStorageAccount()
		networkSettings := storageSettings.GetNetworkSettings()

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

		resourceGroupName := pulumi.Sprintf("rg-%s-storage-%s", strings.ToLower(projectName), strings.ToLower(envShort))
		storageRg, err := resources.NewResourceGroup(ctx, "storage_rg", &resources.ResourceGroupArgs{
			ResourceGroupName: resourceGroupName,
			Location:          pulumi.String(s.getDefaultParams().GetRegion().shortString()),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// if storage account enabled in template
		if s.GetRequest().GetStorageAccount().GetDisabled() == false {
			var publicNetworkAccessString string
			if networkSettings.GetPublicNetworkEnabled() {
				publicNetworkAccessString = "enabled"
			} else {
				publicNetworkAccessString = "disabled"
			}

			stAccount, err := storage.NewStorageAccount(ctx, fmt.Sprintf("st%sdata", strings.ToLower(projectName)), &storage.StorageAccountArgs{
				ResourceGroupName:     storageRg.Name,
				IsHnsEnabled:          pulumi.Bool(storageSettings.GetHnsEnabled()),
				AllowBlobPublicAccess: pulumi.Bool(storageSettings.GetBlobAnonAccessEnabled()),
				PublicNetworkAccess:   pulumi.String(publicNetworkAccessString),
				Kind:                  pulumi.String(storageSettings.GetKind().shortString()),
				Sku:                   storage.SkuArgs{Name: pulumi.String(storageSettings.GetSku().shortString())},
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}

			// create the private endpoints for the storage account
			if networkSettings.GetPrivateEndpoint().GetEnabled() {
				for _, sub := range networkSettings.GetPrivateEndpoint().GetSubResources() {
					// TODO: #2 figure out the private dns zone record thing
					pe, err := network.NewPrivateEndpoint(ctx, fmt.Sprintf("storage_account_%s_pe", sub), &network.PrivateEndpointArgs{
						ResourceGroupName:   storageRg.Name,
						PrivateEndpointName: pulumi.Sprintf("%s-%s-pe", stAccount.Name, sub),
						Subnet: network.SubnetTypeArgs{
							Id: pulumi.String(subnetId),
						},
						PrivateLinkServiceConnections: network.PrivateLinkServiceConnectionArray{network.PrivateLinkServiceConnectionArgs{
							Name:                 pulumi.Sprintf("%s_connection", sub),
							PrivateLinkServiceId: stAccount.ID(),
							GroupIds:             pulumi.StringArray{pulumi.String(sub)},
						}},
					}, pulumi.Provider(provider))
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
			}

			ctx.Export("storageAccountName", stAccount.Name)
			ctx.Export("storageAccountId", stAccount.ID())
		} else {
			ctx.Export("storageAccountName", pulumi.String(""))
			ctx.Export("storageAccountId", pulumi.String(""))
		}

		// TODO: i think I can make the private endpoint object better here - like implement the pulumi.Input interface on my custom type
		// private endpoint info export
		var convertedEndpoints []any
		for _, p := range privateEndpoints {
			convertedEndpoints = append(convertedEndpoints, p)
		}
		ctx.Export("privateEndpoints", pulumi.ToArray(convertedEndpoints))
		ctx.Export("resourceGroupName", storageRg.Name)
		ctx.Export("resourceGroupID", storageRg.ID())

		return nil
	}
}

func (s StorAcctSKU) shortString() string {
	switch s {
	case StorAcctSKU_STOR_ACCT_SKU_STANDARD_LRS:
		return "Standard_LRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARD_ZRS:
		return "Standard_ZRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARD_GRS:
		return "Standard_GRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARD_GZRS:
		return "Standard_GZRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARD_RAGRS:
		return "Standard_RAGRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARD_RAGZRS:
		return "Standard_RAGZRS"
	case StorAcctSKU_STOR_ACCT_SKU_PREMIUM_LRS:
		return "Premium_LRS"
	case StorAcctSKU_STOR_ACCT_SKU_PREMIUM_ZRS:
		return "Premium_ZRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_LRS:
		return "StandardV2_LRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_ZRS:
		return "StandardV2_ZRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_GRS:
		return "StandardV2_GRS"
	case StorAcctSKU_STOR_ACCT_SKU_STANDARDV2_GZRS:
		return "StandardV2_GZRS"
	case StorAcctSKU_STOR_ACCT_SKU_PREMIUMV2_LRS:
		return "PremiumV2_LRS"
	case StorAcctSKU_STOR_ACCT_SKU_PREMIUMV2_ZRS:
		return "PremiumV2_ZRS"
	default:
		return "Standard_LRS"
	}
}

func (s StorAcctKind) shortString() string {
	switch s {
	case StorAcctKind_STOR_ACCT_KIND_STORAGE_V2:
		return "StorageV2"
	case StorAcctKind_STOR_ACCT_KIND_BLOCK_BLOB_STORAGE:
		return "BlockBlobStorage"
	case StorAcctKind_STOR_ACCT_KIND_FILE_STORAGE:
		return "FileStorage"
	default:
		return "StorageV2"
	}
}
