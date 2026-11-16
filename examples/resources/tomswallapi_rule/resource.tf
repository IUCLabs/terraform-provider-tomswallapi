resource "tomswallapi_rule" "ssh_loc_to_dmz" {
  action = "accept"
  source = ["loc"]
  dest   = ["dmz"]
  proto  = "tcp"
  ports  = ["22"]
}

resource "tomswallapi_rule" "loc_to_cloudflare" {
  action    = "accept"
  source    = ["loc"]
  dest      = ["net:+asn_cloudflare"]
  portgroup = "https"
}
