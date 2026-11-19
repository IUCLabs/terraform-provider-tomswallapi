resource "tomswallapi_routing_rule" "office_via_isp1" {
  device           = "fw1"
  source           = "198.51.100.0/24"
  routing_provider = "isp1"
  priority         = 1000
}
