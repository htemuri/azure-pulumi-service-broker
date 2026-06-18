package main

type Environment int

const (
	DEV Environment = iota
	TEST
	STAGING
	PRODUCTION
)

var envName = map[Environment]string{
	DEV:        "dev",
	TEST:       "tst",
	STAGING:    "stg",
	PRODUCTION: "prod",
}

func (ee Environment) String() string {
	return envName[ee]
}

type Config struct {
	PulumiClientId     string
	PulumiClientSecret string

	TenantId                       string
	BillingScope                   string
	ClientProjectManagementGroupId string
	Region                         string

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
