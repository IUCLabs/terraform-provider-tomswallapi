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
	_ resource.Resource                = &stoppedRuleResource{}
	_ resource.ResourceWithImportState = &stoppedRuleResource{}
)

type stoppedRuleResource struct{ client *apiClient }

type stoppedRuleModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Device  types.String `tfsdk:"device"`
	Action  types.String `tfsdk:"action"`
	Source  types.String `tfsdk:"source"`
	Dest    types.String `tfsdk:"dest"`
	Proto   types.String `tfsdk:"proto"`
	DPort   types.List   `tfsdk:"dport"`
	SPort   types.List   `tfsdk:"sport"`
	Comment types.String `tfsdk:"comment"`
}

type stoppedRuleAPI struct {
	ID      int64    `json:"id,omitempty"`
	Device  string   `json:"device"`
	Action  string   `json:"action"`
	Source  string   `json:"source,omitempty"`
	Dest    string   `json:"dest,omitempty"`
	Proto   string   `json:"proto,omitempty"`
	DPort   []string `json:"dport,omitempty"`
	SPort   []string `json:"sport,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

func NewStoppedRuleResource() resource.Resource { return &stoppedRuleResource{} }

func (r *stoppedRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stopped_rule"
}

func (r *stoppedRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Traffic permitted when the firewall is stopped, on a device.",
		Attributes: map[string]schema.Attribute{
			"id":      schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":  schema.StringAttribute{Description: "Owning device.", Required: true},
			"action":  schema.StringAttribute{Description: "accept, drop, ...", Required: true},
			"source":  schema.StringAttribute{Optional: true},
			"dest":    schema.StringAttribute{Optional: true},
			"proto":   schema.StringAttribute{Optional: true},
			"dport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"sport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"comment": schema.StringAttribute{Optional: true},
		},
	}
}

func (r *stoppedRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *stoppedRuleResource) body(ctx context.Context, plan stoppedRuleModel, diags *diag.Diagnostics) stoppedRuleAPI {
	return stoppedRuleAPI{
		Device:  plan.Device.ValueString(),
		Action:  plan.Action.ValueString(),
		Source:  plan.Source.ValueString(),
		Dest:    plan.Dest.ValueString(),
		Proto:   plan.Proto.ValueString(),
		DPort:   listToStrings(ctx, plan.DPort, diags),
		SPort:   listToStrings(ctx, plan.SPort, diags),
		Comment: plan.Comment.ValueString(),
	}
}

func (r *stoppedRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stoppedRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out stoppedRuleAPI
	if err := r.client.post(ctx, "/api/v1/stopped-rules", body, &out); err != nil {
		resp.Diagnostics.AddError("create stopped_rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *stoppedRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state stoppedRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out stoppedRuleAPI
	if err := r.client.post(ctx, "/api/v1/stopped-rules", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate stopped_rule failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/stopped-rules/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old stopped_rule failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *stoppedRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state stoppedRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out stoppedRuleAPI
	if err := r.client.get(ctx, "/api/v1/stopped-rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read stopped_rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *stoppedRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state stoppedRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/stopped-rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete stopped_rule failed", err.Error())
	}
}

func (r *stoppedRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "stopped_rule id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *stoppedRuleResource) toModel(ctx context.Context, api stoppedRuleAPI, prior stoppedRuleModel, diags *diag.Diagnostics) stoppedRuleModel {
	return stoppedRuleModel{
		ID:      types.Int64Value(api.ID),
		Device:  types.StringValue(api.Device),
		Action:  types.StringValue(api.Action),
		Source:  optionalString(api.Source, prior.Source),
		Dest:    optionalString(api.Dest, prior.Dest),
		Proto:   optionalString(api.Proto, prior.Proto),
		DPort:   optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort:   optionalList(ctx, api.SPort, prior.SPort, diags),
		Comment: optionalString(api.Comment, prior.Comment),
	}
}
