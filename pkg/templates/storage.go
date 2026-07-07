package templates

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (sr *StorageRequest) newTemplate() (Template, error) {
	s := &Storage{
		StackName:        fmt.Sprintf("%s-storage", sr.GetDefaultParams().GetEnvironment().ShortString()),
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
	if baseResponse.GetSubscriptionId() == "" || baseResponse.GetVnetId() == "" {
		return &newResponse, fmt.Errorf("missing base template response when trying to deploy security template")
	}
	// subnets, ok := cm["subnets"].(SubnetResponse)
	// if !ok {
	// 	return cm, fmt.Errorf("failed getting 'subnets' from cm")
	// }
	stack.SetConfig(ctx, "subscriptionId", auto.ConfigValue{Value: baseResponse.SubscriptionId})
	stack.SetConfig(ctx, "vnetId", auto.ConfigValue{Value: baseResponse.VnetId})

	_, err = stack.Up(
		ctx,
		debugOptions,
		streamer)
	if err != nil {
		return &newResponse, fmt.Errorf("failed to update stack: %v\n\n", err)
	}
	return &newResponse, nil
}

func (s *Storage) pulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		if subscriptionId, ok := ctx.GetConfig("subscriptionId"); ok {
			ctx.Log.Info(fmt.Sprintf("got subscriptionId: %s", subscriptionId), nil)
		} else {
			ctx.Log.Info("failed to get subscriptionId", nil)
		}
		return nil
	}
}
