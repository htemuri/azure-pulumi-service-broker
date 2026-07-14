> [!NOTE]
> **AI Disclaimer**
> 
> This repo _almost_[^1] does not contain any AI generated code or ideation as it's a project meant for learning. The documentation and write-up is written with the assistance of an LLM.
>
> [^1]: Except for the `unwrapOneof` function [here](https://github.com/htemuri/azure-pulumi-service-broker/blob/d2a7459dee45592ecf3dd7986f28de51d6590c13/pkg/templates/template.go#L43). I couldn't figure out how to properly cast the [`isTemplates_Template` oneOf](https://github.com/htemuri/azure-pulumi-service-broker/blob/d2a7459dee45592ecf3dd7986f28de51d6590c13/pkg/templates/template.pb.go#L32) interface in my generated protobuf code to my custom interface for Template without having to manually write cases for each possible template that implemented `isTemplates_Template`.

# azure-pulumi-service-broker
A self-service API to provision Azure objects using custom Pulumi templates[^2] instead of Terraform







### Potential Improvements
  - [ ] nats authentication and tls
  - [ ] maybe a helm chart to deploy the entire stack to k8s
  - [ ] attaching telemetry to the bundle
  - [x] different "templates" for how projects are structured? like make project top level into resource groups instead of subscriptions
  - [ ] rollback feature?
  - [ ] option to export stack to files that can be pushed to a repo that keep it in sync
    - [ ] option to choose format of export [eg: terraform hcl, pulumi stack, arm, json, yaml?]
  - [ ] switch to pulumi templates stored on repos with pulumi.runFuncs and protobuf definitions for config args
  - [ ] separate dns zone creation
  - [ ] separate entra object and iam access provisioning


[^2] Not actual pulumi templates as defined [here](https://www.pulumi.com/templates/). I wanted to use actual templates, but I was limited by the pulumi automation api's current capabilities (feature request for it can be found in [this issue](https://github.com/pulumi/pulumi/issues/16949).)