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
	_ resource.Resource                = &tcPriorityResource{}
	_ resource.ResourceWithImportState = &tcPriorityResource{}
)

type tcPriorityResource struct{ client *apiClient }

type tcPriorityModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Device    types.String `tfsdk:"device"`
	Band      types.Int64  `tfsdk:"band"`
	Proto     types.String `tfsdk:"proto"`
	DPort     types.List   `tfsdk:"dport"`
	SPort     types.List   `tfsdk:"sport"`
	Address   types.String `tfsdk:"address"`
	Interface types.String `tfsdk:"interface"`
	Helper    types.String `tfsdk:"helper"`
	Comment   types.String `tfsdk:"comment"`
}

type tcPriorityAPI struct {
	ID        int64    `json:"id,omitempty"`
	Device    string   `json:"device"`
	Band      int      `json:"band"`
	Proto     string   `json:"proto,omitempty"`
	DPort     []string `json:"dport,omitempty"`
	SPort     []string `json:"sport,omitempty"`
	Address   string   `json:"address,omitempty"`
	Interface string   `json:"interface,omitempty"`
	Helper    string   `json:"helper,omitempty"`
	Comment   string   `json:"comment,omitempty"`
}

func NewTCPriorityResource() resource.Resource { return &tcPriorityResource{} }

func (r *tcPriorityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tc_priority"
}

func (r *tcPriorityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A simple priority-band classification on a device.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":    schema.StringAttribute{Description: "Owning device.", Required: true},
			"band":      schema.Int64Attribute{Description: "Priority band (1, 2, or 3).", Required: true},
			"proto":     schema.StringAttribute{Description: "Protocol to match (tcp, udp, icmp, ...).", Optional: true},
			"dport":     schema.ListAttribute{Description: "Destination ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"sport":     schema.ListAttribute{Description: "Source ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"address":   schema.StringAttribute{Description: "Source address to match.", Optional: true},
			"interface": schema.StringAttribute{Description: "Ingress interface to match.", Optional: true},
			"helper":    schema.StringAttribute{Description: "Conntrack helper the packets must be associated with.", Optional: true},
			"comment":   schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *tcPriorityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *tcPriorityResource) body(ctx context.Context, plan tcPriorityModel, diags *diag.Diagnostics) tcPriorityAPI {
	return tcPriorityAPI{
		Device: plan.Device.ValueString(), Band: int(plan.Band.ValueInt64()), Proto: plan.Proto.ValueString(),
		DPort: listToStrings(ctx, plan.DPort, diags), SPort: listToStrings(ctx, plan.SPort, diags),
		Address: plan.Address.ValueString(), Interface: plan.Interface.ValueString(),
		Helper: plan.Helper.ValueString(), Comment: plan.Comment.ValueString(),
	}
}

func (r *tcPriorityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tcPriorityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcPriorityAPI
	if err := r.client.post(ctx, "/api/v1/tc-priorities", body, &out); err != nil {
		resp.Diagnostics.AddError("create tc_priority failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *tcPriorityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tcPriorityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcPriorityAPI
	if err := r.client.post(ctx, "/api/v1/tc-priorities", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate tc_priority failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/tc-priorities/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old tc_priority failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *tcPriorityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tcPriorityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcPriorityAPI
	if err := r.client.get(ctx, "/api/v1/tc-priorities/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read tc_priority failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *tcPriorityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tcPriorityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/tc-priorities/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete tc_priority failed", err.Error())
	}
}

func (r *tcPriorityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "tc_priority id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *tcPriorityResource) toModel(ctx context.Context, api tcPriorityAPI, prior tcPriorityModel, diags *diag.Diagnostics) tcPriorityModel {
	return tcPriorityModel{
		ID: types.Int64Value(api.ID), Device: types.StringValue(api.Device), Band: types.Int64Value(int64(api.Band)),
		Proto: optionalString(api.Proto, prior.Proto), DPort: optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort: optionalList(ctx, api.SPort, prior.SPort, diags), Address: optionalString(api.Address, prior.Address),
		Interface: optionalString(api.Interface, prior.Interface), Helper: optionalString(api.Helper, prior.Helper),
		Comment: optionalString(api.Comment, prior.Comment),
	}
}
