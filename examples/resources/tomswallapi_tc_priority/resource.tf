resource "tomswallapi_tc_priority" "ssh" {
  device = "fw1"
  band   = 1
  proto  = "tcp"
  dport  = ["22"]
}
