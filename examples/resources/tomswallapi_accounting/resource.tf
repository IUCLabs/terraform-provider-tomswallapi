resource "tomswallapi_accounting" "web_in" {
  device = "fw1"
  action = "COUNT"
  dest   = "192.0.2.10"
  proto  = "tcp"
  dport  = ["443"]
}
