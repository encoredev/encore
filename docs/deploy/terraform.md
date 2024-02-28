---
seotitle: Integrate Encore with existing infrastructure
seodesc: The Encore terraform provider lets you integrate your encore deployment with existing infrastructure
title: Terraform Provider
subtitle: Integrate Encore with existing infrastructure
infobox: {
  title: "Terraform Provider",
  import: "https://registry.terraform.io/providers/encoredev/encore",
}
---
Encore simplifies the deployment and management of cloud applications. When working with complex systems, you often 
need to integrate Encore-provisioned resources into your broader infrastructure landscape. Terraform data sources 
offer a powerful mechanism for bridging this gap.

## Understanding Encore Terraform Data Sources

Encore Terraform data sources act as read-only references to resources Encore has already provisioned on your behalf. 
Unlike Terraform resources (which create or modify infrastructure), data sources only retrieve information. The Encore
data sources let's you retrieve cloud identifiers for resources managed by Encore, such as databases, caches, and more.
To do this, you only need to provide the name of the resource and the environment it's in.

## Configuring the Encore Terraform Provider

To use Encore data sources, you need to declare the Encore Terraform provider in the `required_providers` of
your Terraform configuration file. Here's an example of how to declare the provider:

```terraform
terraform {
  required_providers {
    encore = {
      source = "registry.terraform.io/encoredev/encore"
    }
  }
}
```

Once you've declared the provider, Terraform will automatically download the provider plugin when initializing the 
working directory using `terraform init`.

To authenticate with the Encore API, the provider need an Encore Auth Key. You can generate an auth key using the
encore [cloud dashboard](https://encore.dev/docs/develop/auth-keys). Once you have the auth key, you can configure the
provider in your Terraform configuration file like this:

```terraform
provider "encore" {
    env = "your-env"
    auth_key = "your-auth-key"
}
```
You can also set the `ENCORE_AUTH_KEY` environment variable to avoid hardcoding the auth key in your configuration file.

## Using Encore Terraform Data Sources

Once you have the provider configured, you can use the encore data sources to retrieve information about resources. 
There are several data sources available, such as `encore_database`, `encore_cache`, and `encore_pubsub_topic`. Each data
source has its own set of attributes that you can use to retrieve information about the resource. The full documentation
for each data source is available in the [Terraform Registry](https://registry.terraform.io/providers/encoredev/encore).

Here's an example of how to use the `encore_pubsub_topic` data source to connect AWS IOT Core to an Encore PubSub topic:

```terraform
data "encore_pubsub_topic" "topic" {
  name = "my-topic"
  env  = "my-env"
}

resource "aws_iot_topic_rule" "rule" {
  name = "my-rule"
  sql  = "SELECT * FROM 'my-topic'"
  sns {
    message_format = "RAW"
    role_arn       = aws_iam_role.role.arn
    target_arn     = data.encore_pubsub_topic.topic.aws_sns.arn
  }
}
```

