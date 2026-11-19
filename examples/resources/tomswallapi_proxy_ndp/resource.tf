resource "tomswallapi_proxy_ndp" "web" {
  device    = "fw1"
  address   = "2001:db8::10"
  interface = "eth1"
  external  = "eth0"
}
