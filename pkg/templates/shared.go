package templates

import "fmt"

type vnetUsageResponse struct {
	CurrentValue int    `json:"currentValue"`
	Id           string `json:"id"`
	Limit        int    `json:"limit"`
}

// writing a custom shortstring functions because you can't serialize enums to custom strings in protobuf
func (e Environment) shortString() string {
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
func (r Region) shortString() string {
	switch r {
	case Region_REGION_EASTUS:
		return "EASTUS"
	case Region_REGION_EASTUS2:
		return "EASTUS2"
	default:
		return "unspecified"
	}
}

func (c *PulumiProviderCredentialArgs) validate() (bool, error) {
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
