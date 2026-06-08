package template

type Project struct {
	Name             string `json:"name"`
	Users            map[Role][]User
	Groups           map[Role]Group
	ServicePrincipal ServicePrincipalOptions
	StorageAccount   StorageAccountOptions
	KeyVault         KeyVaultOptions
	DataFactory      DataFactoryOptions
}

type Role int

const (
	Admin Role = iota
	Developer
	Reader
)

type Roles struct {
	Admins     []User
	Developers []User
	Readers    []User
}

type User struct {
	UserPrincipalName string
	ObjectId          string
}

type Group struct {
	DisplayName string
	ObjectId    string
}

type StorageAccountOptions struct {
	Enabled bool
}

type KeyVaultOptions struct {
	Enabled bool
}

type ServicePrincipalOptions struct {
	Enabled bool
}

type DataFactoryOptions struct {
	Enabled bool
}
