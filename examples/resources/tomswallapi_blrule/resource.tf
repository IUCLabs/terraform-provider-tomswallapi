resource "tomswallapi_blrule" "block_scanner" {
  action = "drop"
  source = "net:203.0.113.66"
  log    = "info"
}
