package templates

import (
	"context"
	"fmt"
	"os"

	templates "github.com/htemuri/azure-pulumi-service-broker/gen/go/templates/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func NewStorageTemplate(projectName string, environment templates.Environment, region templates.Region, cred *templates.PulumiProviderCredentialArgs) (*templates.Storage, error) {
	s := templates.Storage{
		DefaultParams: &templates.DefaultParams{
			ProjectName:              projectName,
			Environment:              environment,
			Region:                   region,
			PulumiProviderCredential: cred,
		},
	}
	err := s.Validate()
	if err != nil {
		return &templates.Storage{}, err
	}

	return &s, nil
}

func (s *Storage) Hash() templates.TemplateOptions {
	return TemplateOptions_TEMPLATE_OPTIONS_STORAGE
}

func (s *templates.Storage) GetProjectName() string {
	return s.DefaultParams.GetProjectName()
}

func (s *Storage) GetStackName() string {
	return fmt.Sprintf("%s-storage", s.GetDefaultParams().Environment.ShortString())
}

func (s *Storage) GetProviders() []*templates.ProviderVersion {
	return []*templates.ProviderVersion{{ProviderName: "azure-native", Version: "v3.19.0"}}
}

func (s *Storage) GetDependsOn() []templates.TemplateOptions {
	return []templates.TemplateOptions{TemplateOptions_TEMPLATE_OPTIONS_BASE, TemplateOptions_TEMPLATE_OPTIONS_SECURITY}
}

func (s *Storage) Validate() error {
	if s == nil {
		return fmt.Errorf("Storage can't be nil")
	}
	d, err := GetValidDefaultParams(s)
	if err != nil {
		return err
	}
	if d.Region == Region_REGION_UNSPECIFIED {
		d.Region = Region_REGION_EASTUS
	}
	return nil
}

func (s *Storage) Deploy(ctx context.Context, cm map[string]any, autonamingConfig map[string]string) (map[string]any, error) {
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
	fmt.Println(cm["subnets"])
	subnets, ok := cm["subnets"].(SubnetResponse)
	if !ok {
		return cm, fmt.Errorf("failed getting 'subnets' from cm")
	}

	stack.SetConfig(ctx, "subscriptionId", auto.ConfigValue{Value: subId})
	stack.SetConfig(ctx, "vnetId", auto.ConfigValue{Value: vnetId})
	stack.SetConfig(ctx, "subnetName", auto.ConfigValue{Value: subnets[0].Name})
	stack.SetConfig(ctx, "subnetId", auto.ConfigValue{Value: subnets[0].ID})

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

func (s *Storage) PulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		if subscriptionId, ok := ctx.GetConfig("subscriptionId"); ok {
			ctx.Log.Info(fmt.Sprintf("got subscriptionId: %s", subscriptionId), nil)
		} else {
			ctx.Log.Info("failed to get subscriptionId", nil)
		}
		return nil
	}
}
