package templates

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func NewSecurityTemplate(projectName string, environment Environment, region Region, keyvaultArgs *KeyVaultArgs, cred *PulumiProviderCredentialArgs) (*Security, error) {
	s := Security{
		DefaultParams: &DefaultParams{
			Enabled:                  true,
			ProjectName:              projectName,
			Environment:              environment,
			Region:                   region,
			PulumiProviderCredential: cred,
		},
		KeyVault: keyvaultArgs,
	}
	err := s.Validate()
	if err != nil {
		return &Security{}, err
	}

	return &s, nil
}

func (s *Security) Hash() TemplateOptions {
	return TemplateOptions_TEMPLATE_OPTIONS_SECURITY
}

func (s *Security) GetProjectName() string {
	return s.DefaultParams.GetProjectName()
}

func (s *Security) GetStackName() string {
	return fmt.Sprintf("%s-security", s.GetDefaultParams().Environment.ShortString())
}

func (s *Security) GetProviders() []*ProviderVersion {
	return []*ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}}
}

func (s *Security) GetDependsOn() []TemplateOptions {
	return []TemplateOptions{TemplateOptions_TEMPLATE_OPTIONS_BASE}
}

func (s *Security) Validate() error {
	if s == nil {
		return fmt.Errorf("security can't be nil")
	}
	d, err := GetValidDefaultParams(s)
	if err != nil {
		return err
	}
	if d.Region == Region_REGION_UNSPECIFIED {
		d.Region = Region_REGION_EASTUS
	}

	if s.GetKeyVault() != nil && s.GetKeyVault().GetNetworkSettings() != nil {
		pa := s.KeyVault.NetworkSettings.GetPrivateEndpoint()
		// if private endpoint settings are specified
		if pa != nil && pa.Enabled == true {
			for _, r := range pa.SubResources {
				if r != "vault" {
					return fmt.Errorf("keyvault only supports vault subresources")
				}
			}
		}
	}
	return nil
}

func (s *Security) Deploy(ctx context.Context, cm map[string]any, autonamingConfig map[string]string) (map[string]any, error) {
	stack, err := createOrSelectStack(s, ctx, autonamingConfig)
	if err != nil {
		return cm, err
	}
	// extract config values
	subId, ok := cm["subscriptionId"].(string)
	if !ok {
		return cm, fmt.Errorf("failed getting 'subscriptionId' from cm")
	}
	vnetId, ok := cm["vnetId"].(string)
	if !ok {
		return cm, fmt.Errorf("failed getting 'vnetId' from cm")
	}
	// subnets, ok := cm["subnets"].(SubnetResponse)
	// if !ok {
	// 	return cm, fmt.Errorf("failed getting 'subnets' from cm")
	// }
	stack.SetConfig(ctx, "subscriptionId", auto.ConfigValue{Value: subId})
	stack.SetConfig(ctx, "vnetId", auto.ConfigValue{Value: vnetId})
	// stack.SetConfig(ctx, "subnetName", auto.ConfigValue{Value: subnets[0].Name})
	// stack.SetConfig(ctx, "subnetId", auto.ConfigValue{Value: subnets[0].ID})

	// // 1 - 11 (least verbose to most verbose)
	logLevel := uint(2)

	debugLogging := optup.DebugLogging(debug.LoggingOptions{
		LogToStdErr:   true,
		LogLevel:      &logLevel,
		FlowToPlugins: true,
		Debug:         true,
	})

	streamer := optup.ProgressStreams(os.Stdout)
	_, err = stack.Up(
		ctx,
		debugLogging,
		streamer)
	if err != nil {
		return cm, fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	return cm, nil
}

func (s *Security) PulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		if subscriptionId := conf.Get("subscriptionId"); subscriptionId != "" {
			ctx.Log.Info(fmt.Sprintf("got subscriptionId: %s", subscriptionId), nil)
		} else {
			ctx.Log.Info("failed to get subscriptionId", nil)
		}
		return nil
	}
}
