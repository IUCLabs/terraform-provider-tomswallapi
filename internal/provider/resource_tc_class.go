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
	_ resource.Resource                = &tcClassResource{}
	_ resource.ResourceWithImportState = &tcClassResource{}
)

type tcClassResource struct{ client *apiClient }

type tcClassModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Device    types.String `tfsdk:"device"`
	Interface types.String `tfsdk:"interface"`
	Mark      types.Int64  `tfsdk:"mark"`
	Rate      types.String `tfsdk:"rate"`
	Ceil      types.String `tfsdk:"ceil"`
	Priority  types.Int64  `tfsdk:"priority"`
	Comment   types.String `tfsdk:"comment"`
}

type tcClassAPI struct {
	ID        int64  `json:"id,omitempty"`
	Device    string `json:"device"`
	Interface string `json:"interface"`
	Mark      int    `json:"mark,omitempty"`
	Rate      string `json:"rate,omitempty"`
	Ceil      string `json:"ceil,omitempty"`
	Priority  int    `json:"priority,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

func NewTCClassResource() resource.Resource { return &tcClassResource{} }

func (r *tcClassResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tc_class"
}

func (r *tcClassResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A traffic-shaping class on a device.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":    schema.StringAttribute{Description: "Owning device.", Required: true},
			"interface": schema.StringAttribute{Description: "iface:class or iface:parent:class.", Required: true},
			"mark":      schema.Int64Attribute{Description: "Firewall mark that classifies traffic into this class.", Optional: true},
			"rate":      schema.StringAttribute{Description: "Guaranteed rate (e.g. 10mbit).", Optional: true},
			"ceil":      schema.StringAttribute{Description: "Maximum rate (e.g. 100mbit, or full).", Optional: true},
			"priority":  schema.Int64Attribute{Description: "Class priority; lower first.", Optional: true},
			"comment":   schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *tcClassResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *tcClassResource) body(plan tcClassModel) tcClassAPI {
	return tcClassAPI{
		Device: plan.Device.ValueString(), Interface: plan.Interface.ValueString(), Mark: int(plan.Mark.ValueInt64()),
		Rate: plan.Rate.ValueString(), Ceil: plan.Ceil.ValueString(), Priority: int(plan.Priority.ValueInt64()),
		Comment: plan.Comment.ValueString(),
	}
}

func (r *tcClassResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tcClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcClassAPI
	if err := r.client.post(ctx, "/api/v1/tc-classes", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create tc_class failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *tcClassResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tcClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcClassAPI
	if err := r.client.post(ctx, "/api/v1/tc-classes", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate tc_class failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/tc-classes/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old tc_class failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *tcClassResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tcClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out tcClassAPI
	if err := r.client.get(ctx, "/api/v1/tc-classes/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read tc_class failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *tcClassResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tcClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/tc-classes/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete tc_class failed", err.Error())
	}
}

func (r *tcClassResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "tc_class id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *tcClassResource) toModel(api tcClassAPI, prior tcClassModel) tcClassModel {
	return tcClassModel{
		ID: types.Int64Value(api.ID), Device: types.StringValue(api.Device), Interface: types.StringValue(api.Interface),
		Mark: optionalInt64(api.Mark, prior.Mark), Rate: optionalString(api.Rate, prior.Rate),
		Ceil: optionalString(api.Ceil, prior.Ceil), Priority: optionalInt64(api.Priority, prior.Priority),
		Comment: optionalString(api.Comment, prior.Comment),
	}
}
