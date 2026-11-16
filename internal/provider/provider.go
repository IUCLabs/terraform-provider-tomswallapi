package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &tomswallProvider{}

type tomswallProvider struct {
	version string
}

type tomswallProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

// New returns the provider constructor.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &tomswallProvider{version: version} }
}

func (p *tomswallProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tomswallapi"
	resp.Version = p.version
}

func (p *tomswallProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a tomswallapi firewall control plane: fleet-global zones, address groups, portgroups, rules, policies, and fabrics, plus per-device zone->interface bindings.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "The tomswallapi base URL (e.g. https://tomswallapi.example.com).",
				Required:    true,
			},
			"token": schema.StringAttribute{
				Description: "Write token (bearer) for mutating operations. Defaults to the TOMSWALLAPI_WRITE_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *tomswallProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config tomswallProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := config.Token.ValueString()
	if token == "" {
		token = os.Getenv("TOMSWALLAPI_WRITE_TOKEN")
	}

	client := newAPIClient(config.Endpoint.ValueString(), token)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *tomswallProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewZoneResource,
		NewFabricResource,
		NewPortGroupResource,
		NewAddressGroupResource,
		NewDeviceResource,
		NewBindingResource,
		NewRuleResource,
		NewSNATResource,
		NewNetmapResource,
		NewNATResource,
		NewPolicyResource,
		NewBlruleResource,
		NewConntrackResource,
		NewHostResource,
		NewProviderResource,
		NewRouteResource,
		NewRoutingRuleResource,
		NewTunnelResource,
		NewStoppedRuleResource,
		NewProxyARPResource,
		NewProxyNDPResource,
		NewArpRuleResource,
		NewMaclistResource,
		NewMangleResource,
		NewAccountingResource,
		NewTCDeviceResource,
		NewTCClassResource,
		NewTCFilterResource,
		NewTCInterfaceResource,
		NewTCPriorityResource,
		NewSecmarkResource,
		NewVarResource,
	}
}

func (p *tomswallProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
