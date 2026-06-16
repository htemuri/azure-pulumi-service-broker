> [!NOTE]
> This repo does not contain any AI generated code as it's a project meant to learn and practice.

# azure-pulumi-service-broker
A self-service API to provision Azure objects using Pulumi instead of Terraform

## TODO

- [ ] Pulumi worker 
  - [ ] resource provisioning code
    - [ ] networking 
    - [x] storage acct
    - [x] akv
    - [ ] entra id principals
    - [ ] adf
    - [x] auth pulumi with service principal
  - [x] nats async subject streaming
  - [ ] nats app logging and io writing pulumi events
- [ ] nats server
  - [ ] init script
  - [ ] worker queue
- [ ] Broker API
  - [ ] start with rest endpoints
  - [x] nats publish handlers
- [ ] Packaging it all
  - [ ] dockerfiles for each service
  - [ ] maybe a helm chart to deploy the entire stack to k8s
- [ ] Improvements
  - [ ] nats authentication and tls
  - [ ] attaching telemetry to the bundle
  - [ ] different "templates" for how projects are structured? like make project top level into resource groups instead of subscriptions
  - [ ] rollback feature?
  - [ ] option to export stack to files that can be pushed to a repo that keep it in sync
    - [ ] option to choose format of export [eg: terraform hcl, pulumi stack, arm, json, yaml?]