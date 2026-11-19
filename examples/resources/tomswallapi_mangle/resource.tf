resource "tomswallapi_mangle" "mark_voip" {
  device     = "fw1"
  action     = "MARK"
  mark_value = "1"
  proto      = "udp"
  dport      = ["5060"]
}
