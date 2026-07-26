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

// proxyResource backs both tomswallapi_proxy_arp and tomswallapi_proxy_ndp
// (identical shape); typeSuffix and endpoint distinguish them.
type proxyResource struct {
	client     *apiClient
	typeSuffix string
	endpoint   string
}

func NewProxyARPResource() resource.Resource {
	return &proxyResource{typeSuffix: "_proxy_arp", endpoint: "/api/v1/proxy-arp"}
}
func NewProxyNDPResource() resource.Resource {
	return &proxyResource{typeSuffix: "_proxy_ndp", endpoint: "/api/v1/proxy-ndp"}
}

var (
	_ resource.Resource                = &proxyResource{}
	_ resource.ResourceWithImportState = &proxyResource{}
)

type proxyModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Device     types.String `tfsdk:"device"`
	Address    types.String `tfsdk:"address"`
	Interface  types.String `tfsdk:"interface"`
	External   types.String `tfsdk:"external"`
	HaveRoute  types.Bool   `tfsdk:"haveroute"`
	Persistent types.Bool   `tfsdk:"persistent"`
	Comment    types.String `tfsdk:"comment"`
}

type proxyAPI struct {
	ID         int64  `json:"id,omitempty"`
	Device     string `json:"device"`
	Address    string `json:"address"`
	Interface  string `json:"interface,omitempty"`
	External   string `json:"external"`
	HaveRoute  bool   `json:"haveroute,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

func (r *proxyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *proxyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Answers ARP/NDP on behalf of another host (proxy arp/ndp), on a device.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":     schema.StringAttribute{Description: "Owning device.", Required: true},
			"address":    schema.StringAttribute{Required: true},
			"interface":  schema.StringAttribute{Optional: true},
			"external":   schema.StringAttribute{Description: "External interface.", Required: true},
			"haveroute":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"persistent": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"comment":    schema.StringAttribute{Optional: true},
		},
	}
}

func (r *proxyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *proxyResource) body(plan proxyModel) proxyAPI {
	return proxyAPI{
		Device:     plan.Device.ValueString(),
		Address:    plan.Address.ValueString(),
		Interface:  plan.Interface.ValueString(),
		External:   plan.External.ValueString(),
		HaveRoute:  plan.HaveRoute.ValueBool(),
		Persistent: plan.Persistent.ValueBool(),
		Comment:    plan.Comment.ValueString(),
	}
}

func (r *proxyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan proxyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out proxyAPI
	if err := r.client.post(ctx, r.endpoint, r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create proxy failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *proxyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state proxyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out proxyAPI
	if err := r.client.post(ctx, r.endpoint, r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate proxy failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, r.endpoint+"/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old proxy failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *proxyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state proxyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out proxyAPI
	if err := r.client.get(ctx, r.endpoint+"/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read proxy failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *proxyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state proxyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, r.endpoint+"/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete proxy failed", err.Error())
	}
}

func (r *proxyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "proxy id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *proxyResource) toModel(api proxyAPI, prior proxyModel) proxyModel {
	return proxyModel{
		ID:         types.Int64Value(api.ID),
		Device:     types.StringValue(api.Device),
		Address:    types.StringValue(api.Address),
		Interface:  optionalString(api.Interface, prior.Interface),
		External:   types.StringValue(api.External),
		HaveRoute:  types.BoolValue(api.HaveRoute),
		Persistent: types.BoolValue(api.Persistent),
		Comment:    optionalString(api.Comment, prior.Comment),
	}
}
