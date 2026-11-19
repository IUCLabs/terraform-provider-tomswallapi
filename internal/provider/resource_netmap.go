package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &netmapResource{}
	_ resource.ResourceWithImportState = &netmapResource{}
)

type netmapResource struct{ client *apiClient }

type netmapModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Type    types.String `tfsdk:"type"`
	FromNet types.String `tfsdk:"from_net"`
	ToNet   types.String `tfsdk:"to_net"`
	Anchor  types.String `tfsdk:"anchor"`
	Comment types.String `tfsdk:"comment"`
}

type netmapAPI struct {
	ID      int64  `json:"id,omitempty"`
	Type    string `json:"type"`
	FromNet string `json:"from_net"`
	ToNet   string `json:"to_net"`
	Anchor  string `json:"anchor"`
	Comment string `json:"comment,omitempty"`
}

func NewNetmapResource() resource.Resource { return &netmapResource{} }

func (r *netmapResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_netmap"
}

func (r *netmapResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A network-to-network mapping (netmap), anchored at a device:zone or device:interface. It renders on the anchored device with the zone resolved to its bound interface.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description:   "Server-assigned id.",
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Description: "dnat or snat.",
				Required:    true,
			},
			"from_net": schema.StringAttribute{
				Description: "Network (CIDR) to match.",
				Required:    true,
			},
			"to_net": schema.StringAttribute{
				Description: "Network (CIDR) to rewrite to.",
				Required:    true,
			},
			"anchor": schema.StringAttribute{
				Description: "Anchor as device:zone or device:interface.",
				Required:    true,
			},
			"comment": schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *netmapResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *netmapResource) body(plan netmapModel) netmapAPI {
	return netmapAPI{
		Type:    plan.Type.ValueString(),
		FromNet: plan.FromNet.ValueString(),
		ToNet:   plan.ToNet.ValueString(),
		Anchor:  plan.Anchor.ValueString(),
		Comment: plan.Comment.ValueString(),
	}
}

func (r *netmapResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan netmapModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out netmapAPI
	if err := r.client.post(ctx, "/api/v1/netmap", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create netmap failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

// Update: the netmap API is create/delete only, so recreate and delete the old id.
func (r *netmapResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state netmapModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out netmapAPI
	if err := r.client.post(ctx, "/api/v1/netmap", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate netmap failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/netmap/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old netmap failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *netmapResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state netmapModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out netmapAPI
	if err := r.client.get(ctx, "/api/v1/netmap/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read netmap failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *netmapResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state netmapModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/netmap/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete netmap failed", err.Error())
	}
}

func (r *netmapResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "netmap id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *netmapResource) toModel(api netmapAPI, prior netmapModel) netmapModel {
	return netmapModel{
		ID:      types.Int64Value(api.ID),
		Type:    types.StringValue(api.Type),
		FromNet: types.StringValue(api.FromNet),
		ToNet:   types.StringValue(api.ToNet),
		Anchor:  types.StringValue(api.Anchor),
		Comment: optionalString(api.Comment, prior.Comment),
	}
}
