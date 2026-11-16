package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &tunnelResource{}
	_ resource.ResourceWithImportState = &tunnelResource{}
)

type tunnelResource struct{ client *apiClient }

type tunnelModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Device       types.String `tfsdk:"device"`
	Type         types.String `tfsdk:"type"`
	Zone         types.String `tfsdk:"zone"`
	Gateways     types.List   `tfsdk:"gateways"`
	GatewayZones types.List   `tfsdk:"gateway_zones"`
	Port         types.Int64  `tfsdk:"port"`
	Comment      types.String `tfsdk:"comment"`
}

type tunnelAPI struct {
	ID           int64    `json:"id,omitempty"`
	Device       string   `json:"device"`
	Type         string   `json:"type"`
	Zone         string   `json:"zone"`
	Gateways     []string `json:"gateways,omitempty"`
	GatewayZones []string `json:"gateway_zones,omitempty"`
	Port         int      `json:"port,omitempty"`
	Comment      string   `json:"comment,omitempty"`
}

func NewTunnelResource() resource.Resource { return &tunnelResource{} }

func (r *tunnelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tunnel"
}

func (r *tunnelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A VPN tunnel definition on a device (allows encapsulated traffic).",
		Attributes: map[string]schema.Attribute{
			"id":            schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":        schema.StringAttribute{Description: "Owning device.", Required: true},
			"type":          schema.StringAttribute{Description: "Tunnel type (ipsec, openvpn:udp, ...).", Required: true},
			"zone":          schema.StringAttribute{Description: "Zone the remote tunnel endpoint is in.", Required: true},
			"gateways":      schema.ListAttribute{Description: "Remote gateway addresses.", Optional: true, ElementType: types.StringType},
			"gateway_zones": schema.ListAttribute{Description: "Zones the remote gateway may be in.", Optional: true, ElementType: types.StringType},
			"port":          schema.Int64Attribute{Description: "Port the tunnel uses, overriding the type default.", Optional: true},
			"comment":       schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *tunnelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *tunnelResource) body(ctx context.Context, plan tunnelModel, diags *diag.Diagnostics) tunnelAPI {
	return tunnelAPI{
		Device:       plan.Device.ValueString(),
		Type:         plan.Type.ValueString(),
		Zone:         plan.Zone.ValueString(),
		Gateways:     listToStrings(ctx, plan.Gateways, diags),
		GatewayZones: listToStrings(ctx, plan.GatewayZones, diags),
		Port:         int(plan.Port.ValueInt64()),
		Comment:      plan.Comment.ValueString(),
	}
}

func (r *tunnelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tunnelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tunnelAPI
	if err := r.client.post(ctx, "/api/v1/tunnels", body, &out); err != nil {
		resp.Diagnostics.AddError("create tunnel failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *tunnelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tunnelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tunnelAPI
	if err := r.client.post(ctx, "/api/v1/tunnels", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate tunnel failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/tunnels/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old tunnel failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *tunnelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tunnelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tunnelAPI
	if err := r.client.get(ctx, "/api/v1/tunnels/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read tunnel failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *tunnelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tunnelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/tunnels/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete tunnel failed", err.Error())
	}
}

func (r *tunnelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "tunnel id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *tunnelResource) toModel(ctx context.Context, api tunnelAPI, prior tunnelModel, diags *diag.Diagnostics) tunnelModel {
	return tunnelModel{
		ID:           types.Int64Value(api.ID),
		Device:       types.StringValue(api.Device),
		Type:         types.StringValue(api.Type),
		Zone:         types.StringValue(api.Zone),
		Gateways:     optionalList(ctx, api.Gateways, prior.Gateways, diags),
		GatewayZones: optionalList(ctx, api.GatewayZones, prior.GatewayZones, diags),
		Port:         optionalInt64(api.Port, prior.Port),
		Comment:      optionalString(api.Comment, prior.Comment),
	}
}
