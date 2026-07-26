# terraform-provider-tomswallapi

Terraform/OpenTofu provider for [tomswallapi](https://git.unkin.net/unkin/tomswallapi),
the fleet control plane for tomswall. Declare the fleet-global firewall model —
zones, address groups, portgroups, rules, and fabrics — plus per-device
zone→interface bindings, as HCL.

## Provider configuration

```hcl
terraform {
  required_providers {
    tomswallapi = {
      source = "git.unkin.net/unkin/tomswallapi"
    }
  }
}

provider "tomswallapi" {
  endpoint = "https://tomswallapi.k8s.syd1.au.unkin.net"
  # token defaults to the TOMSWALLAPI_WRITE_TOKEN environment variable
}
```

## Resources

| resource | key | notes |
|---|---|---|
| `tomswallapi_zone` | name | fleet-global segment; `subnets` may be empty for edge zones |
| `tomswallapi_address_group` | name | `type` = static/dns/asn; asn `resolved`/`resolved_at` are computed |
| `tomswallapi_portgroup` | name | reusable proto+ports |
| `tomswallapi_fabric` | name | `enforce_on_routers` toggles defense-in-depth |
| `tomswallapi_device` | name | `class` = router/firewall, `fabric`, `resolver`, `settings` |
| `tomswallapi_binding` | device:zone | zone→interface map (import as `device:zone`) |
| `tomswallapi_rule` | id | shorewall-style `source`/`dest` element lists |
| `tomswallapi_snat` | id | masquerade/SNAT; `source` zone-or-CIDR, `egress` zone |
| `tomswallapi_netmap` | id | net-to-net map, anchored `device:zone`\|`device:interface` |
| `tomswallapi_nat` | id | 1:1 static NAT bound to a `device` |
| `tomswallapi_policy` | id | default zone-to-zone posture (`priority` ordered) |
| `tomswallapi_blrule` | id | blacklist/whitelist rule (pre-rules) |
| `tomswallapi_conntrack` | id | connection-tracking control (notrack/helper) |
| `tomswallapi_host` | id | zone→address constraint on a `device` interface |
| `tomswallapi_provider` | id | multi-ISP routing provider on a `device` |
| `tomswallapi_route` | id | static route on a `device` (`oif` = egress iface) |
| `tomswallapi_routing_rule` | id | policy routing to a provider table (rtrules) |
| `tomswallapi_tunnel` | id | VPN tunnel definition on a `device` |
| `tomswallapi_stopped_rule` | id | traffic allowed while the firewall is stopped |
| `tomswallapi_proxy_arp` / `_proxy_ndp` | id | proxy ARP/NDP on a `device` |
| `tomswallapi_arp_rule` | id | ARP-level rule on a `device` |
| `tomswallapi_maclist` | id | MAC/IP verification on a `device` interface |

## Example

See [`examples/`](examples/). A minimal A→cloudflare rule:

```hcl
resource "tomswallapi_zone" "loc" { name = "loc" subnets = ["10.1.0.0/24"] }
resource "tomswallapi_zone" "net" { name = "net" }  # edge zone, no subnets

resource "tomswallapi_address_group" "cloudflare" {
  name    = "cloudflare"
  type    = "asn"
  members = ["13335"]
  refresh = "24h"
}

resource "tomswallapi_rule" "cf_https" {
  action = "accept"
  source = ["loc"]
  dest   = ["net:+asn_cloudflare"]
  proto  = "tcp"
  ports  = ["443"]
}
```

## Development

```sh
make build     # build the provider binary
make install   # install into ~/.terraform.d/plugins for local testing
make test      # go test
make lint      # go vet
```

## Releases

`make patch|minor|major` tags and pushes the next `v*`, which triggers the
Woodpecker release pipeline to package the provider and publish it to the
artifactapi terraform registry (`terraform-unkin`), installable via the bare
`source = "git.unkin.net/unkin/tomswallapi"` address.
