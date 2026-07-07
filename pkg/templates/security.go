package templates

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func (sr *SecurityRequest) newTemplate() (Template, error) {
	s := &Security{
		StackName:        fmt.Sprintf("%s-security", sr.GetDefaultParams().GetEnvironment().ShortString()),
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
			for _, r := range pa.SubResources {
				if r != "vault" {
					return fmt.Errorf("keyvault only supports vault subresources")
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
	if baseResponse.GetSubscriptionId() == "" || baseResponse.GetVnetId() == "" {
		return &newResponse, fmt.Errorf("missing base template response when trying to deploy security template")
	}
	// subnets, ok := cm["subnets"].(SubnetResponse)
	// if !ok {
	// 	return cm, fmt.Errorf("failed getting 'subnets' from cm")
	// }
	stack.SetConfig(ctx, "subscriptionId", auto.ConfigValue{Value: baseResponse.SubscriptionId})
	stack.SetConfig(ctx, "vnetId", auto.ConfigValue{Value: baseResponse.VnetId})
	// stack.SetConfig(ctx, "subnetName", auto.ConfigValue{Value: subnets[0].Name})
	// stack.SetConfig(ctx, "subnetId", auto.ConfigValue{Value: subnets[0].ID})

	_, err = stack.Up(
		ctx,
		debugOptions,
		streamer)
	if err != nil {
		return &newResponse, fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	return &newResponse, nil
}

func (s *Security) pulumiRunFunc() pulumi.RunFunc {
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
