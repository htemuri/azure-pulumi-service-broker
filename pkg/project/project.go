package project

import (
	reflect "reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// writing a custom shortstring functions because you can't serialize enums to custom strings in protobuf
func (s StorageAccountSubResource) ShortString() string {
	switch s {
	case StorageAccountSubResource_STORAGE_ACCOUNT_SUB_RESOURCE_BLOB:
		return "blob"
	case StorageAccountSubResource_STORAGE_ACCOUNT_SUB_RESOURCE_DFS:
		return "dfs"
	case StorageAccountSubResource_STORAGE_ACCOUNT_SUB_RESOURCE_QUEUE:
		return "queue"
	case StorageAccountSubResource_STORAGE_ACCOUNT_SUB_RESOURCE_FILE:
		return "file"
	case StorageAccountSubResource_STORAGE_ACCOUNT_SUB_RESOURCE_WEB:
		return "web"
	default:
		return "unspecified"
	}
}

func (s RoleType) ShortString() string {
	switch s {
	case RoleType_ROLE_TYPE_ADMIN:
		return "admin"
	case RoleType_ROLE_TYPE_DEVELOPER:
		return "developer"
	case RoleType_ROLE_TYPE_READER:
		return "reader"
	default:
		return "unspecified"
	}
}

// // might be multiple entries for users and groups with duplicate role types
// func (p *Project) RoleUserList(roleType RoleType) []*User {
// 	var resultList []*User
// 	for _, v := range p.GetUsers() {
// 		if v.Role == roleType {
// 			resultList = append(resultList, v.GetUsers()...)
// 		}
// 	}
// 	return resultList
// }

// func (p *Project) RoleGroupList(roleType RoleType) []*Group {
// 	var resultList []*Group
// 	for _, v := range p.GetGroups() {
// 		if v.Role == roleType {
// 			resultList = append(resultList, v.Group)
// 		}
// 	}
// 	return resultList
// }

// TODO: this doesn't work. need to figure out how to implement pulumi.Input properly and pass that to the pulumi run func in the worker service.
type PrivateEndpointPulumiExport struct {
	Fqdn        pulumi.Input
	IpAddress   pulumi.Input
	DnsZoneName pulumi.Input
}

var privateEndpointExportType = reflect.TypeFor[map[string]any]()

func (PrivateEndpointPulumiExport) ElementType() reflect.Type {
	return privateEndpointExportType
}
