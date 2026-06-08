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

type GlobalVars struct {
	TenantId                       string
	BillingScope                   string
	ClientProjectManagementGroupId string
	Region                         string
}
