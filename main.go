// Command terraform-provider-tomswallapi is the Terraform/OpenTofu provider for
// tomswallapi: it manages the fleet-global firewall model (zones, address groups,
// portgroups, rules, policies, fabrics) and per-device zone->interface bindings.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"git.unkin.net/unkin/terraform-provider-tomswallapi/internal/provider"
)

var version = "0.0.1"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "git.unkin.net/unkin/tomswallapi",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err)
	}
}
