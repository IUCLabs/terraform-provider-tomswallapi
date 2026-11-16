resource "tomswallapi_snat" "masq_loc" {
  action = "masquerade"
  source = "loc"
  egress = "net"
}
