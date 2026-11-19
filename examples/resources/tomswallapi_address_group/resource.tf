resource "tomswallapi_address_group" "cloudflare" {
  name    = "cloudflare"
  type    = "asn"
  members = ["13335"]
  refresh = "24h"
}

resource "tomswallapi_address_group" "office" {
  name    = "office"
  type    = "static"
  members = ["198.51.100.0/24"]
}
