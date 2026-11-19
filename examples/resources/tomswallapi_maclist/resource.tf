resource "tomswallapi_maclist" "printer" {
  device    = "fw1"
  action    = "accept"
  interface = "eth1"
  mac       = "02:00:00:00:00:01"
  addresses = ["192.0.2.50"]
}
