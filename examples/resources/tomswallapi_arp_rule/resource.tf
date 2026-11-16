resource "tomswallapi_arp_rule" "drop_spoof" {
  device = "fw1"
  action = "DROP"
  source = "eth1:192.0.2.1"
  opcode = 2
}
