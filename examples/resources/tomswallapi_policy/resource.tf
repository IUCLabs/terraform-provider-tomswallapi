resource "tomswallapi_policy" "loc_to_net" {
  source   = "loc"
  dest     = "net"
  action   = "accept"
  priority = 10
}

resource "tomswallapi_policy" "default_drop" {
  source   = "all"
  dest     = "all"
  action   = "drop"
  log      = "info"
  priority = 100
}
