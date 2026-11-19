resource "tomswallapi_conntrack" "ftp_helper" {
  action = "helper"
  helper = "ftp"
  proto  = "tcp"
  dport  = ["21"]
}
