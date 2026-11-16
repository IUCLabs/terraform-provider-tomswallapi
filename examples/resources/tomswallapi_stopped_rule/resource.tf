resource "tomswallapi_stopped_rule" "admin_ssh" {
  device = "fw1"
  action = "ACCEPT"
  source = "eth1:192.0.2.0/24"
  proto  = "tcp"
  dport  = ["22"]
}
