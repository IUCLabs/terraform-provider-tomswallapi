resource "tomswallapi_provider" "isp1" {
  device    = "fw1"
  name      = "isp1"
  number    = 1
  mark      = 1
  duplicate = "main"
  interface = "eth0"
  gateway   = "detect"
}
