package templates

import "fmt"

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
func (e Environment) ShortString() string {
	switch e {
	case Environment_ENVIRONMENT_DEV:
		return "dev"
	case Environment_ENVIRONMENT_TST:
		return "tst"
	case Environment_ENVIRONMENT_STG:
		return "stg"
	case Environment_ENVIRONMENT_PRD:
		return "prd"
	default:
		return "unspecified"
	}
}
func (r Region) ShortString() string {
	switch r {
	case Region_REGION_EASTUS:
		return "EASTUS"
	case Region_REGION_EASTUS2:
		return "EASTUS2"
	default:
		return "unspecified"
	}
}

func (c *PulumiProviderCredentialArgs) Validate() (bool, error) {
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
