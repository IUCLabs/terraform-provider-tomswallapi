resource "tomswallapi_tc_device" "wan" {
  device        = "fw1"
  interface     = "eth0"
  in_bandwidth  = "100mbit"
  out_bandwidth = "20mbit"
}
