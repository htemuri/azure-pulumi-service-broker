package main

type Config struct {
	// Nats
	ProjectStreamName string
}

// resource naming configs
var autonamingConfig = map[string]string{
	"resources:ResourceGroup": "${name}",
	"network:VirtualNetwork":  "${name}-${num(3)}",
	"storage:StorageAccount":  "${name}${num(3)}",
	"keyvault:Vault":          "${name}-${num(3)}",
	"datafactory:Factory":     "${name}-${num(3)}",
}
