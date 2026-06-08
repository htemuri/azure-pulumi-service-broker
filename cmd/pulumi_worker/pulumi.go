package main

import (
	"fmt"
	"strings"

	"github.com/htemuri/azure-pulumi-service-broker/pkg/template"
	pulumiazurenativesdk "github.com/pulumi/pulumi-azure-native-sdk"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/subscription/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func runPulumiJob(env Environment, project template.Project, globalVars GlobalVars) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		sub, err := subscription.NewAlias(ctx, env.String(), &subscription.AliasArgs{
			Properties: subscription.PutAliasRequestPropertiesArgs{
				DisplayName:  pulumi.String(fmt.Sprintf("[%s] Client Project: %s", strings.ToUpper(env.String()), strings.ToUpper(project.Name))),
				BillingScope: pulumi.String(globalVars.BillingScope),
				Workload:     pulumi.String("Production"),
				AdditionalProperties: subscription.PutAliasRequestAdditionalPropertiesArgs{
					ManagementGroupId: pulumi.String(globalVars.ClientProjectManagementGroupId),
				},
			},
		})
		if err != nil {
			return err
		}

		provider, err := pulumiazurenativesdk.NewProvider(ctx, "new_sub_provider", &pulumiazurenativesdk.ProviderArgs{
			SubscriptionId: sub.Properties.SubscriptionId(),
			TenantId:       pulumi.String(globalVars.TenantId),
		})

		rg, err := resources.NewResourceGroup(ctx, "network", &resources.ResourceGroupArgs{
			Location: pulumi.String(globalVars.Region),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}
		ctx.Export("rgName", rg.Name)
		ctx.Export("subscriptionId", sub.Properties.SubscriptionId())
		return nil
	}
}
