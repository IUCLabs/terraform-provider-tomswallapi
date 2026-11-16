package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &hostResource{}
	_ resource.ResourceWithImportState = &hostResource{}
)

type hostResource struct{ client *apiClient }

type hostModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Device     types.String `tfsdk:"device"`
	Zone       types.String `tfsdk:"zone"`
	Interface  types.String `tfsdk:"interface"`
	Addresses  types.List   `tfsdk:"addresses"`
	Exclusions types.List   `tfsdk:"exclusions"`
	Dynamic    types.Bool   `tfsdk:"dynamic"`
}

type hostAPI struct {
	ID         int64    `json:"id,omitempty"`
	Device     string   `json:"device"`
	Zone       string   `json:"zone"`
	Interface  string   `json:"interface"`
	Addresses  []string `json:"addresses,omitempty"`
	Exclusions []string `json:"exclusions,omitempty"`
	Dynamic    bool     `json:"dynamic,omitempty"`
}

func NewHostResource() resource.Resource { return &hostResource{} }

func (r *hostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (r *hostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Constrains a zone to specific addresses on a device's interface.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":     schema.StringAttribute{Description: "Owning device.", Required: true},
			"zone":       schema.StringAttribute{Description: "Zone the addresses belong to.", Required: true},
			"interface":  schema.StringAttribute{Description: "Device interface the addresses are reached through.", Required: true},
			"addresses":  schema.ListAttribute{Description: "Addresses or networks on the interface that belong to the zone.", Optional: true, ElementType: types.StringType},
			"exclusions": schema.ListAttribute{Description: "Addresses excluded from the zone.", Optional: true, ElementType: types.StringType},
			"dynamic":    schema.BoolAttribute{Description: "Make zone membership dynamic so hosts can be added at runtime.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		},
	}
}

func (r *hostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *hostResource) body(ctx context.Context, plan hostModel, diags *diag.Diagnostics) hostAPI {
	return hostAPI{
		Device:     plan.Device.ValueString(),
		Zone:       plan.Zone.ValueString(),
		Interface:  plan.Interface.ValueString(),
		Addresses:  listToStrings(ctx, plan.Addresses, diags),
		Exclusions: listToStrings(ctx, plan.Exclusions, diags),
		Dynamic:    plan.Dynamic.ValueBool(),
	}
}

func (r *hostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out hostAPI
	if err := r.client.post(ctx, "/api/v1/hosts", body, &out); err != nil {
		resp.Diagnostics.AddError("create host failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *hostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state hostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out hostAPI
	if err := r.client.post(ctx, "/api/v1/hosts", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate host failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/hosts/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old host failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *hostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out hostAPI
	if err := r.client.get(ctx, "/api/v1/hosts/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read host failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *hostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/hosts/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete host failed", err.Error())
	}
}

func (r *hostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "host id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *hostResource) toModel(ctx context.Context, api hostAPI, prior hostModel, diags *diag.Diagnostics) hostModel {
	return hostModel{
		ID:         types.Int64Value(api.ID),
		Device:     types.StringValue(api.Device),
		Zone:       types.StringValue(api.Zone),
		Interface:  types.StringValue(api.Interface),
		Addresses:  optionalList(ctx, api.Addresses, prior.Addresses, diags),
		Exclusions: optionalList(ctx, api.Exclusions, prior.Exclusions, diags),
		Dynamic:    types.BoolValue(api.Dynamic),
	}
}
