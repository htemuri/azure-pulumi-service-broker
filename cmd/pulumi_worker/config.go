package main

type Environment int

const (
	Environment_ENTRA Environment = iota
	Environment_DEV
	Environment_TEST
	Environment_STAGING
	Environment_PRODUCTION
)

var envName = map[Environment]string{
	Environment_ENTRA:      "entra",
	Environment_DEV:        "dev",
	Environment_TEST:       "tst",
	Environment_STAGING:    "stg",
	Environment_PRODUCTION: "prod",
}

func (ee Environment) String() string {
	return envName[ee]
}

type Config struct {
	PulumiClientId                   string
	PulumiClientSecret               string
	PulumiAzureADProviderVersion     string
	PulumiAzureNativeProviderVersion string

	TenantId                       string
	BillingScope                   string
	ClientProjectManagementGroupId string
	Region                         string

	// Nats
	ProjectStreamName string

	// Networking stuff
	ClientDevVnetIpAllocId string

	// Entra
	EntraIdAdminObjectIds []string
}

// resource naming configs

var autonamingConfig = map[string]string{
	"resources:ResourceGroup": "${name}",
	"network:VirtualNetwork":  "${name}-${num(3)}",
	"storage:StorageAccount":  "${name}${num(3)",
	"keyvault:Vault":          "${name}-${num(3)}",
	"datafactory:Factory":     "${name}-${num(3)}",
}
