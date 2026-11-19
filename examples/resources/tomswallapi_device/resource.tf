resource "tomswallapi_device" "fw1" {
  name   = "fw1"
  class  = "firewall"
  fabric = "core"

  settings = {
    log_level = "info"
  }
}
