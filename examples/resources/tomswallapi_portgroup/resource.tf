resource "tomswallapi_portgroup" "https" {
  name  = "https"
  proto = "tcp"
  ports = ["443"]
}
