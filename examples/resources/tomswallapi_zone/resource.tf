resource "tomswallapi_zone" "loc" {
  name    = "loc"
  subnets = ["192.0.2.0/24"]
}

# Internet-facing zone with no subnets; rules pair it with a selector.
resource "tomswallapi_zone" "net" {
  name = "net"
}
