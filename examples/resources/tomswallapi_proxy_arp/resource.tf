resource "tomswallapi_proxy_arp" "web" {
  device    = "fw1"
  address   = "203.0.113.10"
  interface = "eth1"
  external  = "eth0"
}
