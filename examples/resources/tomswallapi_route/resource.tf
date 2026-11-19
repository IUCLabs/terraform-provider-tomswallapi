resource "tomswallapi_route" "lab" {
  device  = "fw1"
  dest    = "198.51.100.0/24"
  gateway = "192.0.2.254"
}
