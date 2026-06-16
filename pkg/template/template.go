package template

// writing a custom shortstring function because you can't serialize enums to custom strings in protobuf
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
