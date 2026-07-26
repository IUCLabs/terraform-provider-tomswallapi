package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &policyResource{}
	_ resource.ResourceWithImportState = &policyResource{}
)

type policyResource struct{ client *apiClient }

type policyModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Priority types.Int64  `tfsdk:"priority"`
	Source   types.String `tfsdk:"source"`
	Dest     types.String `tfsdk:"dest"`
	Action   types.String `tfsdk:"action"`
	Log      types.String `tfsdk:"log"`
}

type policyAPI struct {
	ID       int64  `json:"id,omitempty"`
	Priority int    `json:"priority"`
	Source   string `json:"source"`
	Dest     string `json:"dest"`
	Action   string `json:"action"`
	Log      string `json:"log,omitempty"`
}

func NewPolicyResource() resource.Resource { return &policyResource{} }

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A default zone-to-zone policy (fleet posture). Lower priority is evaluated first.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"priority": schema.Int64Attribute{
				Description: "Evaluation priority; lower first.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
			},
			"source": schema.StringAttribute{Description: "Source zone (or all).", Required: true},
			"dest":   schema.StringAttribute{Description: "Dest zone (or all).", Required: true},
			"action": schema.StringAttribute{Description: "accept, drop, reject, continue, none, ...", Required: true},
			"log":    schema.StringAttribute{Optional: true},
		},
	}
}

func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *policyResource) body(plan policyModel) policyAPI {
	return policyAPI{
		Priority: int(plan.Priority.ValueInt64()),
		Source:   plan.Source.ValueString(),
		Dest:     plan.Dest.ValueString(),
		Action:   plan.Action.ValueString(),
		Log:      plan.Log.ValueString(),
	}
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out policyAPI
	if err := r.client.post(ctx, "/api/v1/policies", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create policy failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out policyAPI
	if err := r.client.post(ctx, "/api/v1/policies", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate policy failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/policies/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old policy failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out policyAPI
	if err := r.client.get(ctx, "/api/v1/policies/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read policy failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/policies/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete policy failed", err.Error())
	}
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "policy id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *policyResource) toModel(api policyAPI, prior policyModel) policyModel {
	return policyModel{
		ID:       types.Int64Value(api.ID),
		Priority: types.Int64Value(int64(api.Priority)),
		Source:   types.StringValue(api.Source),
		Dest:     types.StringValue(api.Dest),
		Action:   types.StringValue(api.Action),
		Log:      optionalString(api.Log, prior.Log),
	}
}
