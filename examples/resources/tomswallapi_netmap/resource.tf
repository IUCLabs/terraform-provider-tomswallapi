resource "tomswallapi_netmap" "remap" {
  type     = "dnat"
  from_net = "203.0.113.0/28"
  to_net   = "192.0.2.0/28"
  anchor   = "fw1:net"
}
