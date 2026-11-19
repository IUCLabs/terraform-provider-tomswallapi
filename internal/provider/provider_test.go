package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestProviderSchemaValid(t *testing.T) {
	srv := providerserver.NewProtocol6(New("test")())()
	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("%s: %s", d.Summary, d.Detail)
		}
	}
	if len(resp.ResourceSchemas) != 32 {
		t.Errorf("got %d resource schemas, want 32", len(resp.ResourceSchemas))
	}
}
