# Terraform Provider tomswallapi

This provider manages a fleet firewall model through the tomswallapi HTTP API
with Terraform or OpenTofu. Zones, address groups, port groups, rules and
policies are declared once for the whole fleet; per-device resources such as
zone-to-interface bindings, NAT, routing providers and traffic shaping are
attached to individual devices.

See the [provider documentation](docs/index.md) for all resources and arguments.

## Quick Start

```hcl
terraform {
  required_providers {
    tomswallapi = {
      source = "IUCLabs/tomswallapi"
    }
  }
}

provider "tomswallapi" {
  endpoint = "https://tomswallapi.example.com"
  # token defaults to the TOMSWALLAPI_WRITE_TOKEN environment variable
}

resource "tomswallapi_zone" "loc" {
  name    = "loc"
  subnets = ["192.0.2.0/24"]
}

resource "tomswallapi_zone" "dmz" {
  name    = "dmz"
  subnets = ["198.51.100.0/24"]
}

resource "tomswallapi_rule" "ssh" {
  action = "accept"
  source = [tomswallapi_zone.loc.name]
  dest   = [tomswallapi_zone.dmz.name]
  proto  = "tcp"
  ports  = ["22"]
}
```

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/install) 1.x or [OpenTofu](https://opentofu.org/)
- [Go](https://go.dev/doc/install) 1.25+ (to build the provider)

## Development

```sh
make build   # build the provider binary
make test    # run unit tests
make docs    # regenerate docs/ from the schema and examples/
```

## License

[MIT](LICENSE)
