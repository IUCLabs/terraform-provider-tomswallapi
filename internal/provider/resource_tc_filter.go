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
	_ resource.Resource                = &tcFilterResource{}
	_ resource.ResourceWithImportState = &tcFilterResource{}
)

type tcFilterResource struct{ client *apiClient }

type tcFilterModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Device   types.String `tfsdk:"device"`
	Class    types.String `tfsdk:"class"`
	Source   types.String `tfsdk:"source"`
	Dest     types.String `tfsdk:"dest"`
	Proto    types.String `tfsdk:"proto"`
	DPort    types.List   `tfsdk:"dport"`
	SPort    types.List   `tfsdk:"sport"`
	TOS      types.String `tfsdk:"tos"`
	Length   types.Int64  `tfsdk:"length"`
	Priority types.Int64  `tfsdk:"priority"`
	Comment  types.String `tfsdk:"comment"`
}

type tcFilterAPI struct {
	ID       int64    `json:"id,omitempty"`
	Device   string   `json:"device"`
	Class    string   `json:"class"`
	Source   string   `json:"source,omitempty"`
	Dest     string   `json:"dest,omitempty"`
	Proto    string   `json:"proto,omitempty"`
	DPort    []string `json:"dport,omitempty"`
	SPort    []string `json:"sport,omitempty"`
	TOS      string   `json:"tos,omitempty"`
	Length   int      `json:"length,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Comment  string   `json:"comment,omitempty"`
}

func NewTCFilterResource() resource.Resource { return &tcFilterResource{} }

func (r *tcFilterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tc_filter"
}

func (r *tcFilterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A traffic-classification filter (packet → class) on a device.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":   schema.StringAttribute{Description: "Owning device.", Required: true},
			"class":    schema.StringAttribute{Description: "interface:class target.", Required: true},
			"source":   schema.StringAttribute{Description: "Source to match (zone, interface, or address in shorewall notation).", Optional: true},
			"dest":     schema.StringAttribute{Description: "Destination to match (zone, interface, or address in shorewall notation).", Optional: true},
			"proto":    schema.StringAttribute{Description: "Protocol to match (tcp, udp, icmp, ...).", Optional: true},
			"dport":    schema.ListAttribute{Description: "Destination ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"sport":    schema.ListAttribute{Description: "Source ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"tos":      schema.StringAttribute{Description: "Type of Service value to match.", Optional: true},
			"length":   schema.Int64Attribute{Description: "Packet length to match.", Optional: true},
			"priority": schema.Int64Attribute{Description: "Filter priority; lower first.", Optional: true},
			"comment":  schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *tcFilterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *tcFilterResource) body(ctx context.Context, plan tcFilterModel, diags *diag.Diagnostics) tcFilterAPI {
	return tcFilterAPI{
		Device: plan.Device.ValueString(), Class: plan.Class.ValueString(), Source: plan.Source.ValueString(),
		Dest: plan.Dest.ValueString(), Proto: plan.Proto.ValueString(),
		DPort: listToStrings(ctx, plan.DPort, diags), SPort: listToStrings(ctx, plan.SPort, diags),
		TOS: plan.TOS.ValueString(), Length: int(plan.Length.ValueInt64()), Priority: int(plan.Priority.ValueInt64()),
		Comment: plan.Comment.ValueString(),
	}
}

func (r *tcFilterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tcFilterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcFilterAPI
	if err := r.client.post(ctx, "/api/v1/tc-filters", body, &out); err != nil {
		resp.Diagnostics.AddError("create tc_filter failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *tcFilterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tcFilterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcFilterAPI
	if err := r.client.post(ctx, "/api/v1/tc-filters", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate tc_filter failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/tc-filters/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old tc_filter failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *tcFilterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tcFilterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcFilterAPI
	if err := r.client.get(ctx, "/api/v1/tc-filters/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read tc_filter failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *tcFilterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tcFilterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/tc-filters/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete tc_filter failed", err.Error())
	}
}

func (r *tcFilterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "tc_filter id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *tcFilterResource) toModel(ctx context.Context, api tcFilterAPI, prior tcFilterModel, diags *diag.Diagnostics) tcFilterModel {
	return tcFilterModel{
		ID: types.Int64Value(api.ID), Device: types.StringValue(api.Device), Class: types.StringValue(api.Class),
		Source: optionalString(api.Source, prior.Source), Dest: optionalString(api.Dest, prior.Dest),
		Proto: optionalString(api.Proto, prior.Proto), DPort: optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort: optionalList(ctx, api.SPort, prior.SPort, diags), TOS: optionalString(api.TOS, prior.TOS),
		Length: optionalInt64(api.Length, prior.Length), Priority: optionalInt64(api.Priority, prior.Priority),
		Comment: optionalString(api.Comment, prior.Comment),
	}
}
