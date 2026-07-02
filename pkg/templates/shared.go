package templates

import (
	"fmt"

	templates "github.com/htemuri/azure-pulumi-service-broker/gen/go/templates/v1"
)

type SubnetResponse []struct {
	AddressPrefixes           []string `json:"addressPrefixes"`
	Delegations               []any    `json:"delegations"`
	Etag                      string   `json:"etag"`
	ID                        string   `json:"id"`
	IpamPoolPrefixAllocations []struct {
		AllocatedAddressPrefixes []string `json:"allocatedAddressPrefixes"`
		ID                       string   `json:"id"`
		NumberOfIPAddresses      string   `json:"numberOfIpAddresses"`
	} `json:"ipamPoolPrefixAllocations"`
	Name                              string `json:"name"`
	PrivateEndpointNetworkPolicies    string `json:"privateEndpointNetworkPolicies"`
	PrivateLinkServiceNetworkPolicies string `json:"privateLinkServiceNetworkPolicies"`
	ProvisioningState                 string `json:"provisioningState"`
	Purpose                           string `json:"purpose"`
	Type                              string `json:"type"`
}

// writing a custom shortstring functions because you can't serialize enums to custom strings in protobuf
func (e templates.Environment) ShortString() string {
	switch e {
	case templates.Environment_ENVIRONMENT_DEV:
		return "dev"
	case templates.Environment_ENVIRONMENT_TST:
		return "tst"
	case templates.Environment_ENVIRONMENT_STG:
		return "stg"
	case templates.Environment_ENVIRONMENT_PRD:
		return "prd"
	default:
		return "unspecified"
	}
}
func (r templates.Region) ShortString() string {
	switch r {
	case templates.Region_REGION_EASTUS:
		return "EASTUS"
	case templates.Region_REGION_EASTUS2:
		return "EASTUS2"
	default:
		return "unspecified"
	}
}

func (c *templates.PulumiProviderCredentialArgs) Validate() (bool, error) {
	if c.TenantId == "" {
		return false, fmt.Errorf("credential missing tenant id")
	}
	if c.ClientId == "" {
		return false, fmt.Errorf("credential missing client id")
	}
	if c.ClientSecret == "" {
		return false, fmt.Errorf("credential missing client secret")
	}
	// maybe check if secret actually works for the cred here too
	return true, nil
}
