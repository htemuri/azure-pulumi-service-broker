> [!NOTE]
> **AI Disclaimer**
> 
> This repo *(almost)* does not contain any AI generated code<sup>1</sup>, ideation, nor documentation as it's a project meant to learn and practice building complex systems.
>
> <sup>1</sup>Except for the `unwrapOneof` function [here](https://github.com/htemuri/azure-pulumi-service-broker/blob/d2a7459dee45592ecf3dd7986f28de51d6590c13/pkg/templates/template.go#L43). I couldn't figure out how to properly cast the [`isTemplates_Template` oneOf](https://github.com/htemuri/azure-pulumi-service-broker/blob/d2a7459dee45592ecf3dd7986f28de51d6590c13/pkg/templates/template.pb.go#L32) interface in my generated protobuf code to my custom interface for Template without having to manually write cases for each possible template that implemented `isTemplates_Template`.

# azure-pulumi-service-broker
A self-service API to provision Azure objects using Pulumi instead of Terraform

## TODO

- [ ] Pulumi worker 
  - [x] resource provisioning code
    - [x] networking 
    - [x] storage acct
    - [x] akv
    - [x] entra id principals
    - [x] adf
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
  - [ ] separate dns zone creation
  - [ ] separate entra object and iam access provisioning
