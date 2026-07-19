package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// configureClient extracts the shared *apiClient from provider data, used by
// every resource's Configure.
func configureClient(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *apiClient {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*apiClient)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return nil
	}
	return client
}

// listToStrings converts a Terraform list into a []string. A null/unknown list
// yields nil.
func listToStrings(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// stringsToList converts a []string into a Terraform list of strings, rendering
// nil as an empty list so state stays consistent.
func stringsToList(ctx context.Context, s []string, diags *diag.Diagnostics) types.List {
	if s == nil {
		s = []string{}
	}
	l, d := types.ListValueFrom(ctx, types.StringType, s)
	diags.Append(d...)
	return l
}

// mapToStrings converts a Terraform map into a map[string]string.
func mapToStrings(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

// stringsToMap converts a map[string]string into a Terraform map.
func stringsToMap(ctx context.Context, m map[string]string, diags *diag.Diagnostics) types.Map {
	if m == nil {
		m = map[string]string{}
	}
	v, d := types.MapValueFrom(ctx, types.StringType, m)
	diags.Append(d...)
	return v
}
