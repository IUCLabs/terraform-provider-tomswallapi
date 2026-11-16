resource "tomswallapi_binding" "fw1_loc" {
  device     = "fw1"
  zone       = "loc"
  interfaces = ["eth1"]
}
