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
	_ resource.Resource                = &maclistResource{}
	_ resource.ResourceWithImportState = &maclistResource{}
)

type maclistResource struct{ client *apiClient }

type maclistModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Device    types.String `tfsdk:"device"`
	Action    types.String `tfsdk:"action"`
	Interface types.String `tfsdk:"interface"`
	MAC       types.String `tfsdk:"mac"`
	Addresses types.List   `tfsdk:"addresses"`
	Log       types.String `tfsdk:"log"`
	Comment   types.String `tfsdk:"comment"`
}

type maclistAPI struct {
	ID        int64    `json:"id,omitempty"`
	Device    string   `json:"device"`
	Action    string   `json:"action"`
	Interface string   `json:"interface"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	Log       string   `json:"log,omitempty"`
	Comment   string   `json:"comment,omitempty"`
}

func NewMaclistResource() resource.Resource { return &maclistResource{} }

func (r *maclistResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maclist"
}

func (r *maclistResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A MAC/IP verification entry on a device's interface.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":    schema.StringAttribute{Description: "Owning device.", Required: true},
			"action":    schema.StringAttribute{Description: "accept, drop, ...", Required: true},
			"interface": schema.StringAttribute{Description: "Interface the MAC address is verified on.", Required: true},
			"mac":       schema.StringAttribute{Description: "MAC address to verify.", Optional: true},
			"addresses": schema.ListAttribute{Description: "IP addresses permitted for the MAC address.", Optional: true, ElementType: types.StringType},
			"log":       schema.StringAttribute{Description: "Log level for matching packets (e.g. info).", Optional: true},
			"comment":   schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *maclistResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *maclistResource) body(ctx context.Context, plan maclistModel, diags *diag.Diagnostics) maclistAPI {
	return maclistAPI{
		Device:    plan.Device.ValueString(),
		Action:    plan.Action.ValueString(),
		Interface: plan.Interface.ValueString(),
		MAC:       plan.MAC.ValueString(),
		Addresses: listToStrings(ctx, plan.Addresses, diags),
		Log:       plan.Log.ValueString(),
		Comment:   plan.Comment.ValueString(),
	}
}

func (r *maclistResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan maclistModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out maclistAPI
	if err := r.client.post(ctx, "/api/v1/maclist", body, &out); err != nil {
		resp.Diagnostics.AddError("create maclist failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *maclistResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state maclistModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out maclistAPI
	if err := r.client.post(ctx, "/api/v1/maclist", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate maclist failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/maclist/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old maclist failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *maclistResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state maclistModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out maclistAPI
	if err := r.client.get(ctx, "/api/v1/maclist/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read maclist failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *maclistResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state maclistModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/maclist/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete maclist failed", err.Error())
	}
}

func (r *maclistResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "maclist id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *maclistResource) toModel(ctx context.Context, api maclistAPI, prior maclistModel, diags *diag.Diagnostics) maclistModel {
	return maclistModel{
		ID:        types.Int64Value(api.ID),
		Device:    types.StringValue(api.Device),
		Action:    types.StringValue(api.Action),
		Interface: types.StringValue(api.Interface),
		MAC:       optionalString(api.MAC, prior.MAC),
		Addresses: optionalList(ctx, api.Addresses, prior.Addresses, diags),
		Log:       optionalString(api.Log, prior.Log),
		Comment:   optionalString(api.Comment, prior.Comment),
	}
}
