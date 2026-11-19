resource "tomswallapi_tc_class" "voip" {
  device    = "fw1"
  interface = "eth0:10"
  mark      = 1
  rate      = "2mbit"
  ceil      = "full"
  priority  = 1
}
