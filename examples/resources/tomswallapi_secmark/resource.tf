resource "tomswallapi_secmark" "http" {
  secmark = "system_u:object_r:http_packet_t:s0"
  chain   = "I:N"
  proto   = "tcp"
  dport   = ["80"]
}
