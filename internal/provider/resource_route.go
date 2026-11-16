package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &routeResource{}
	_ resource.ResourceWithImportState = &routeResource{}
)

type routeResource struct{ client *apiClient }

type routeModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Device     types.String `tfsdk:"device"`
	Provider   types.String `tfsdk:"routing_provider"`
	Dest       types.String `tfsdk:"dest"`
	Gateway    types.String `tfsdk:"gateway"`
	Oif        types.String `tfsdk:"oif"`
	Persistent types.Bool   `tfsdk:"persistent"`
	Comment    types.String `tfsdk:"comment"`
}

type routeAPI struct {
	ID         int64  `json:"id,omitempty"`
	Device     string `json:"device"`
	Provider   string `json:"provider,omitempty"`
	Dest       string `json:"dest"`
	Gateway    string `json:"gateway,omitempty"`
	Oif        string `json:"oif,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

func NewRouteResource() resource.Resource { return &routeResource{} }

func (r *routeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route"
}

func (r *routeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A static route on a device. `oif` is the egress interface.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":           schema.StringAttribute{Description: "Owning device.", Required: true},
			"routing_provider": schema.StringAttribute{Description: "Provider whose routing table receives the route; omit for the main table.", Optional: true},
			"dest":             schema.StringAttribute{Description: "Destination network.", Required: true},
			"gateway":          schema.StringAttribute{Description: "Next-hop gateway address.", Optional: true},
			"oif":              schema.StringAttribute{Description: "Egress interface.", Optional: true},
			"persistent":       schema.BoolAttribute{Description: "Keep the entry in place while the firewall is stopped.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"comment":          schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *routeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *routeResource) body(plan routeModel) routeAPI {
	return routeAPI{
		Device:     plan.Device.ValueString(),
		Provider:   plan.Provider.ValueString(),
		Dest:       plan.Dest.ValueString(),
		Gateway:    plan.Gateway.ValueString(),
		Oif:        plan.Oif.ValueString(),
		Persistent: plan.Persistent.ValueBool(),
		Comment:    plan.Comment.ValueString(),
	}
}

func (r *routeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan routeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out routeAPI
	if err := r.client.post(ctx, "/api/v1/routes", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create route failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *routeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state routeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out routeAPI
	if err := r.client.post(ctx, "/api/v1/routes", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate route failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/routes/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old route failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *routeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state routeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out routeAPI
	if err := r.client.get(ctx, "/api/v1/routes/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read route failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *routeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state routeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/routes/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete route failed", err.Error())
	}
}

func (r *routeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "route id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *routeResource) toModel(api routeAPI, prior routeModel) routeModel {
	return routeModel{
		ID:         types.Int64Value(api.ID),
		Device:     types.StringValue(api.Device),
		Provider:   optionalString(api.Provider, prior.Provider),
		Dest:       types.StringValue(api.Dest),
		Gateway:    optionalString(api.Gateway, prior.Gateway),
		Oif:        optionalString(api.Oif, prior.Oif),
		Persistent: types.BoolValue(api.Persistent),
		Comment:    optionalString(api.Comment, prior.Comment),
	}
}
