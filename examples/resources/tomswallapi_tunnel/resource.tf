resource "tomswallapi_tunnel" "site_b" {
  device   = "fw1"
  type     = "ipsec"
  zone     = "net"
  gateways = ["203.0.113.20"]
}
