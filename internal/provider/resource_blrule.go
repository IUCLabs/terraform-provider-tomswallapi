package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &blruleResource{}
	_ resource.ResourceWithImportState = &blruleResource{}
)

type blruleResource struct{ client *apiClient }

type blruleModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Priority types.Int64  `tfsdk:"priority"`
	Action   types.String `tfsdk:"action"`
	Source   types.String `tfsdk:"source"`
	Dest     types.String `tfsdk:"dest"`
	Proto    types.String `tfsdk:"proto"`
	DPort    types.List   `tfsdk:"dport"`
	SPort    types.List   `tfsdk:"sport"`
	Log      types.String `tfsdk:"log"`
	Comment  types.String `tfsdk:"comment"`
}

type blruleAPI struct {
	ID       int64    `json:"id,omitempty"`
	Priority int      `json:"priority"`
	Action   string   `json:"action"`
	Source   string   `json:"source,omitempty"`
	Dest     string   `json:"dest,omitempty"`
	Proto    string   `json:"proto,omitempty"`
	DPort    []string `json:"dport,omitempty"`
	SPort    []string `json:"sport,omitempty"`
	Log      string   `json:"log,omitempty"`
	Comment  string   `json:"comment,omitempty"`
}

func NewBlruleResource() resource.Resource { return &blruleResource{} }

func (r *blruleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blrule"
}

func (r *blruleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A blacklist/whitelist rule, processed before normal rules.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"priority": schema.Int64Attribute{
				Description: "Evaluation priority; lower first.",
				Optional:    true, Computed: true, Default: int64default.StaticInt64(0),
			},
			"action":  schema.StringAttribute{Description: "drop, reject, whitelist, ...", Required: true},
			"source":  schema.StringAttribute{Optional: true},
			"dest":    schema.StringAttribute{Optional: true},
			"proto":   schema.StringAttribute{Optional: true},
			"dport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"sport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"log":     schema.StringAttribute{Optional: true},
			"comment": schema.StringAttribute{Optional: true},
		},
	}
}

func (r *blruleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *blruleResource) body(ctx context.Context, plan blruleModel, diags *diag.Diagnostics) blruleAPI {
	return blruleAPI{
		Priority: int(plan.Priority.ValueInt64()),
		Action:   plan.Action.ValueString(),
		Source:   plan.Source.ValueString(),
		Dest:     plan.Dest.ValueString(),
		Proto:    plan.Proto.ValueString(),
		DPort:    listToStrings(ctx, plan.DPort, diags),
		SPort:    listToStrings(ctx, plan.SPort, diags),
		Log:      plan.Log.ValueString(),
		Comment:  plan.Comment.ValueString(),
	}
}

func (r *blruleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan blruleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out blruleAPI
	if err := r.client.post(ctx, "/api/v1/blrules", body, &out); err != nil {
		resp.Diagnostics.AddError("create blrule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *blruleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state blruleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out blruleAPI
	if err := r.client.post(ctx, "/api/v1/blrules", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate blrule failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/blrules/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old blrule failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *blruleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state blruleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out blruleAPI
	if err := r.client.get(ctx, "/api/v1/blrules/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read blrule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *blruleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state blruleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/blrules/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete blrule failed", err.Error())
	}
}

func (r *blruleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "blrule id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *blruleResource) toModel(ctx context.Context, api blruleAPI, prior blruleModel, diags *diag.Diagnostics) blruleModel {
	return blruleModel{
		ID:       types.Int64Value(api.ID),
		Priority: types.Int64Value(int64(api.Priority)),
		Action:   types.StringValue(api.Action),
		Source:   optionalString(api.Source, prior.Source),
		Dest:     optionalString(api.Dest, prior.Dest),
		Proto:    optionalString(api.Proto, prior.Proto),
		DPort:    optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort:    optionalList(ctx, api.SPort, prior.SPort, diags),
		Log:      optionalString(api.Log, prior.Log),
		Comment:  optionalString(api.Comment, prior.Comment),
	}
}
