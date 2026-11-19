resource "tomswallapi_nat" "web" {
  device    = "fw1"
  external  = "203.0.113.10"
  internal  = "192.0.2.10"
  interface = "eth0"
}
