resource "tomswallapi_host" "dmz" {
  device    = "fw1"
  zone      = "dmz"
  interface = "eth2"
  addresses = ["192.0.2.128/25"]
}
