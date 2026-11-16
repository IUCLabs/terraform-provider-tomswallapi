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
	_ resource.Resource                = &tcInterfaceResource{}
	_ resource.ResourceWithImportState = &tcInterfaceResource{}
)

type tcInterfaceResource struct{ client *apiClient }

type tcInterfaceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Device       types.String `tfsdk:"device"`
	Interface    types.String `tfsdk:"interface"`
	Type         types.String `tfsdk:"type"`
	InBandwidth  types.String `tfsdk:"in_bandwidth"`
	OutBandwidth types.String `tfsdk:"out_bandwidth"`
	Comment      types.String `tfsdk:"comment"`
}

type tcInterfaceAPI struct {
	ID           int64  `json:"id,omitempty"`
	Device       string `json:"device"`
	Interface    string `json:"interface"`
	Type         string `json:"type,omitempty"`
	InBandwidth  string `json:"in_bandwidth,omitempty"`
	OutBandwidth string `json:"out_bandwidth,omitempty"`
	Comment      string `json:"comment,omitempty"`
}

func NewTCInterfaceResource() resource.Resource { return &tcInterfaceResource{} }

func (r *tcInterfaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tc_interface"
}

func (r *tcInterfaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Simple per-interface traffic shaping on a device.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":        schema.StringAttribute{Description: "Owning device.", Required: true},
			"interface":     schema.StringAttribute{Description: "Interface to shape.", Required: true},
			"type":          schema.StringAttribute{Description: "external or internal.", Optional: true},
			"in_bandwidth":  schema.StringAttribute{Description: "Ingress bandwidth limit (e.g. 100mbit).", Optional: true},
			"out_bandwidth": schema.StringAttribute{Description: "Egress bandwidth (e.g. 100mbit).", Optional: true},
			"comment":       schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *tcInterfaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *tcInterfaceResource) body(plan tcInterfaceModel) tcInterfaceAPI {
	return tcInterfaceAPI{
		Device: plan.Device.ValueString(), Interface: plan.Interface.ValueString(), Type: plan.Type.ValueString(),
		InBandwidth: plan.InBandwidth.ValueString(), OutBandwidth: plan.OutBandwidth.ValueString(), Comment: plan.Comment.ValueString(),
	}
}

func (r *tcInterfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tcInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcInterfaceAPI
	if err := r.client.post(ctx, "/api/v1/tc-interfaces", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create tc_interface failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *tcInterfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tcInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcInterfaceAPI
	if err := r.client.post(ctx, "/api/v1/tc-interfaces", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate tc_interface failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/tc-interfaces/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old tc_interface failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *tcInterfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tcInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcInterfaceAPI
	if err := r.client.get(ctx, "/api/v1/tc-interfaces/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read tc_interface failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *tcInterfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tcInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/tc-interfaces/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete tc_interface failed", err.Error())
	}
}

func (r *tcInterfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "tc_interface id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *tcInterfaceResource) toModel(api tcInterfaceAPI, prior tcInterfaceModel) tcInterfaceModel {
	return tcInterfaceModel{
		ID: types.Int64Value(api.ID), Device: types.StringValue(api.Device), Interface: types.StringValue(api.Interface),
		Type: optionalString(api.Type, prior.Type), InBandwidth: optionalString(api.InBandwidth, prior.InBandwidth),
		OutBandwidth: optionalString(api.OutBandwidth, prior.OutBandwidth), Comment: optionalString(api.Comment, prior.Comment),
	}
}
