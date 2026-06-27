package templates

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

func NewBaseTemplate(defaultParams *DefaultParams, subscriptionId string, resourceGroup string, virtualNetwork *NetworkArgs) *Base {
	base := Base{
		DefaultParams:  defaultParams,
		SubscriptionId: &subscriptionId,
		ResourceGroup:  &resourceGroup,
		VirtualNetwork: virtualNetwork,
	}
	return &base
}

func (b *Base) pulumiRunFunc() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		return nil
	}
}
