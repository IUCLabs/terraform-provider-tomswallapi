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
	_ resource.Resource                = &providerResource{}
	_ resource.ResourceWithImportState = &providerResource{}
)

type providerResource struct{ client *apiClient }

type providerModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Device    types.String `tfsdk:"device"`
	Name      types.String `tfsdk:"name"`
	Number    types.Int64  `tfsdk:"number"`
	Mark      types.Int64  `tfsdk:"mark"`
	Duplicate types.String `tfsdk:"duplicate"`
	Interface types.String `tfsdk:"interface"`
	Gateway   types.String `tfsdk:"gateway"`
	Copy      types.List   `tfsdk:"copy"`
}

type providerAPI struct {
	ID        int64    `json:"id,omitempty"`
	Device    string   `json:"device"`
	Name      string   `json:"name"`
	Number    int      `json:"number"`
	Mark      int      `json:"mark,omitempty"`
	Duplicate string   `json:"duplicate,omitempty"`
	Interface string   `json:"interface"`
	Gateway   string   `json:"gateway,omitempty"`
	Copy      []string `json:"copy,omitempty"`
}

func NewProviderResource() resource.Resource { return &providerResource{} }

func (r *providerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider"
}

func (r *providerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A multi-ISP routing provider on a device.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":    schema.StringAttribute{Description: "Owning device.", Required: true},
			"name":      schema.StringAttribute{Description: "Provider name.", Required: true},
			"number":    schema.Int64Attribute{Description: "Provider routing table number.", Required: true},
			"mark":      schema.Int64Attribute{Description: "Firewall mark that routes traffic through this provider.", Optional: true},
			"duplicate": schema.StringAttribute{Description: "Routing table copied into the provider's table (e.g. main).", Optional: true},
			"interface": schema.StringAttribute{Description: "Interface the provider is reached through.", Required: true},
			"gateway":   schema.StringAttribute{Description: "Gateway address, or detect.", Optional: true},
			"copy":      schema.ListAttribute{Description: "Interfaces whose routes are copied into the provider's table.", Optional: true, ElementType: types.StringType},
		},
	}
}

func (r *providerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *providerResource) body(ctx context.Context, plan providerModel, diags *diag.Diagnostics) providerAPI {
	return providerAPI{
		Device:    plan.Device.ValueString(),
		Name:      plan.Name.ValueString(),
		Number:    int(plan.Number.ValueInt64()),
		Mark:      int(plan.Mark.ValueInt64()),
		Duplicate: plan.Duplicate.ValueString(),
		Interface: plan.Interface.ValueString(),
		Gateway:   plan.Gateway.ValueString(),
		Copy:      listToStrings(ctx, plan.Copy, diags),
	}
}

func (r *providerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan providerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out providerAPI
	if err := r.client.post(ctx, "/api/v1/providers", body, &out); err != nil {
		resp.Diagnostics.AddError("create provider failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *providerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state providerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out providerAPI
	if err := r.client.post(ctx, "/api/v1/providers", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate provider failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/providers/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old provider failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *providerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state providerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out providerAPI
	if err := r.client.get(ctx, "/api/v1/providers/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read provider failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *providerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state providerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/providers/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete provider failed", err.Error())
	}
}

func (r *providerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "provider id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *providerResource) toModel(ctx context.Context, api providerAPI, prior providerModel, diags *diag.Diagnostics) providerModel {
	return providerModel{
		ID:        types.Int64Value(api.ID),
		Device:    types.StringValue(api.Device),
		Name:      types.StringValue(api.Name),
		Number:    types.Int64Value(int64(api.Number)),
		Mark:      optionalInt64(api.Mark, prior.Mark),
		Duplicate: optionalString(api.Duplicate, prior.Duplicate),
		Interface: types.StringValue(api.Interface),
		Gateway:   optionalString(api.Gateway, prior.Gateway),
		Copy:      optionalList(ctx, api.Copy, prior.Copy, diags),
	}
}
