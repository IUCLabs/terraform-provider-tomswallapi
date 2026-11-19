resource "tomswallapi_tc_filter" "voip" {
  device = "fw1"
  class  = "eth0:10"
  proto  = "udp"
  dport  = ["5060"]
}
