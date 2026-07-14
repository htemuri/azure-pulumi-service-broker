# Example client using the broker API

## Overview

This example:
1. Creates a grpc client and connects with the rpc definitions in the `broker` protobuf
2. Fills out the arguments to templates it wants to deploy for a project
3. Sends the request to the broker grpc server
4. Polls the broker api for updates to the project deployment request
   1. After recieving a successful status response, it prints the output of the deployment

## Requirements

- The broker service should be running and you should be able to send requests to the broker api on the port it's listening on. You can test this with `nc`; for example, if the api is on localhost listening on port 50051, running `nc -zv localhost 50051` should return something like below if it's successful.
```
Ncat: Version 7.92 ( https://nmap.org/ncat )
Ncat: Connected to ::1:50051.
Ncat: 0 bytes sent, 0 bytes received in 0.00 seconds.
```
- You should have a service principal with a privileged role like `owner` or `contributor` assigned to the scope you're deploying to. If you're creating a new subscription, that assignment should be scoped to the management group you deploy the sub under. If you're selecting an existing subscription, then it should be scoped to the existing sub.
- The base template vnet only supports address allocation via IPAM pools, so you should have an existing ipam pool if you want to create a new vnet and the pulumi provider service principal should have `network contributor` scoped to the ipam pool. If you want to use an existing vnet, the service principal should have `network contributor` scoped to the vnet.

## Running

You can run this example with

```bash
go run examples/client/main.go
```

and an example output is:

```bash
harristemuri@fedora ~/P/azure-pulumi-service-broker (main)> go run examples/client/main.go
2026/07/13 23:20:31 [Broker Client]: sending project deployment request
2026/07/13 23:20:31 [Broker Client]: deployment id: fbf15616-0a8e-48b9-9e8d-638c44eb25a4
2026/07/13 23:20:41 [Broker Client]: current status: STATUS_IN_PROGRESS
2026/07/13 23:20:51 [Broker Client]: current status: STATUS_IN_PROGRESS
2026/07/13 23:21:01 [Broker Client]: current status: STATUS_IN_PROGRESS
2026/07/13 23:21:11 [Broker Client]: current status: STATUS_IN_PROGRESS
2026/07/13 23:21:21 [Broker Client]: current status: STATUS_IN_PROGRESS
2026/07/13 23:22:01 [Broker Client]: completed deployment:
{
  "project_name": "covid",
  "status": 2,
  "template_responses": [
    {
      "status": 2,
      "Response": {
        "Base": {
          "subscription_id": "23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c",
          "vnet_id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-network-prd/providers/Microsoft.Network/virtualNetworks/vnet-covid-prd-eastus-996",
          "subnets": [
            {
              "name": "subnet-default",
              "id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-network-prd/providers/Microsoft.Network/virtualNetworks/vnet-covid-prd-eastus-996/subnets/subnet-default"
            },
            {
              "name": "subnet-second",
              "id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-network-prd/providers/Microsoft.Network/virtualNetworks/vnet-covid-prd-eastus-996/subnets/subnet-second"
            }
          ]
        }
      }
    },
    {
      "status": 2,
      "Response": {
        "Storage": {
          "resource_group_name": "rg-covid-storage-prd",
          "resource_group_id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-storage-prd",
          "storage_account_name": "stcoviddata404",
          "storage_account_id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-storage-prd/providers/Microsoft.Storage/storageAccounts/stcoviddata404",
          "private_endpoints": [
            {
              "dns_zone_name": "privatelink.blob.core.windows.net",
              "fqdn": "stcoviddata404.blob.core.windows.net",
              "ip_address": "10.0.128.68"
            },
            {
              "dns_zone_name": "privatelink.dfs.core.windows.net",
              "fqdn": "stcoviddata404.dfs.core.windows.net",
              "ip_address": "10.0.128.69"
            }
          ]
        }
      }
    },
    {
      "status": 2,
      "Response": {
        "Security": {
          "resource_group_name": "rg-covid-security-prd",
          "resource_group_id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-security-prd",
          "key_vault_id": "/subscriptions/23b1b9f5-6b57-4c00-87d7-7b49d4d88c6c/resourceGroups/rg-covid-security-prd/providers/Microsoft.KeyVault/vaults/kv-covid-prd-369",
          "key_vault_uri": "https://kv-covid-prd-369.vault.azure.net/",
          "private_endpoints": [
            {
              "dns_zone_name": "privatelink.vaultcore.azure.net",
              "fqdn": "kv-covid-prd-369.vault.azure.net",
              "ip_address": "10.0.128.70"
            }
          ]
        }
      }
    }
  ]
}
```