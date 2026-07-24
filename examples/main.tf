terraform {
  required_providers {
    tomswallapi = {
      source = "git.unkin.net/unkin/tomswallapi"
    }
  }
}

provider "tomswallapi" {
  endpoint = "https://tomswallapi.k8s.syd1.au.unkin.net"
  # token from TOMSWALLAPI_WRITE_TOKEN
}

# --- Fabric + zones ---------------------------------------------------------

resource "tomswallapi_fabric" "core" {
  name               = "core"
  enforce_on_routers = true
}

resource "tomswallapi_zone" "zone_a" {
  name    = "zone-a"
  subnets = ["10.1.0.0/24"]
}

resource "tomswallapi_zone" "zone_b" {
  name    = "zone-b"
  subnets = ["10.4.0.0/24"]
}

# Internet-facing zone: no subnets, so rules must pair it with a selector.
resource "tomswallapi_zone" "net" {
  name = "net"
}

# --- Address group (ASN) ----------------------------------------------------

resource "tomswallapi_address_group" "cloudflare" {
  name    = "cloudflare"
  type    = "asn"
  members = ["13335", "209242"]
  refresh = "24h"
}

# --- Portgroup --------------------------------------------------------------

resource "tomswallapi_portgroup" "https" {
  name  = "https"
  proto = "tcp"
  ports = ["443"]
}

# --- Devices + bindings -----------------------------------------------------

resource "tomswallapi_device" "fw_a" {
  name   = "fw-a"
  class  = "firewall"
  fabric = tomswallapi_fabric.core.name
}

resource "tomswallapi_device" "rt1" {
  name   = "rt1"
  class  = "router"
  fabric = tomswallapi_fabric.core.name
}

resource "tomswallapi_binding" "fw_a_zone_a" {
  device     = tomswallapi_device.fw_a.name
  zone       = tomswallapi_zone.zone_a.name
  interfaces = ["eth1"]
}

# --- Rules (declared once, compiled to every device on the path) ------------

# zone-a can reach zone-b on ssh, through every hop.
resource "tomswallapi_rule" "ssh_a_to_b" {
  action = "accept"
  source = [tomswallapi_zone.zone_a.name]
  dest   = [tomswallapi_zone.zone_b.name]
  proto  = "tcp"
  ports  = ["22"]
}

# zone-a to cloudflare's prefixes on https (edge zone gated by the asn set).
resource "tomswallapi_rule" "a_to_cloudflare" {
  action    = "accept"
  source    = [tomswallapi_zone.zone_a.name]
  dest      = ["net:+asn_cloudflare"]
  portgroup = tomswallapi_portgroup.https.name
}

# --- NAT tier ---------------------------------------------------------------

# Masquerade zone-a out the internet edge. Renders on devices binding both
# zone-a and net (i.e. edge firewalls), on their net-facing interface.
resource "tomswallapi_snat" "masq_zone_a" {
  action = "masquerade"
  source = tomswallapi_zone.zone_a.name
  egress = tomswallapi_zone.net.name
}

# 1:1 static NAT of a public IP to an internal host, on fw-a.
resource "tomswallapi_nat" "web" {
  device    = tomswallapi_device.fw_a.name
  external  = "203.0.113.10"
  internal  = "10.1.0.10"
  interface = "eth0"
}

# Network-to-network map anchored at fw-a's net interface.
resource "tomswallapi_netmap" "remap" {
  type     = "dnat"
  from_net = "10.0.0.0/24"
  to_net   = "192.168.1.0/24"
  anchor   = "${tomswallapi_device.fw_a.name}:${tomswallapi_zone.net.name}"
}
