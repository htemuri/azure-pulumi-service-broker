package main

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
