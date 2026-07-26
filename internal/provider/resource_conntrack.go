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
	_ resource.Resource                = &conntrackResource{}
	_ resource.ResourceWithImportState = &conntrackResource{}
)

type conntrackResource struct{ client *apiClient }

type conntrackModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Priority types.Int64  `tfsdk:"priority"`
	Action   types.String `tfsdk:"action"`
	Source   types.String `tfsdk:"source"`
	Dest     types.String `tfsdk:"dest"`
	Proto    types.String `tfsdk:"proto"`
	DPort    types.List   `tfsdk:"dport"`
	SPort    types.List   `tfsdk:"sport"`
	Chain    types.String `tfsdk:"chain"`
	Helper   types.String `tfsdk:"helper"`
	User     types.String `tfsdk:"user"`
	Comment  types.String `tfsdk:"comment"`
}

type conntrackAPI struct {
	ID       int64    `json:"id,omitempty"`
	Priority int      `json:"priority"`
	Action   string   `json:"action"`
	Source   string   `json:"source,omitempty"`
	Dest     string   `json:"dest,omitempty"`
	Proto    string   `json:"proto,omitempty"`
	DPort    []string `json:"dport,omitempty"`
	SPort    []string `json:"sport,omitempty"`
	Chain    string   `json:"chain,omitempty"`
	Helper   string   `json:"helper,omitempty"`
	User     string   `json:"user,omitempty"`
	Comment  string   `json:"comment,omitempty"`
}

func NewConntrackResource() resource.Resource { return &conntrackResource{} }

func (r *conntrackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_conntrack"
}

func (r *conntrackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A connection-tracking control rule (e.g. notrack, or assign a conntrack helper).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"priority": schema.Int64Attribute{
				Description: "Evaluation priority; lower first.",
				Optional:    true, Computed: true, Default: int64default.StaticInt64(0),
			},
			"action":  schema.StringAttribute{Description: "notrack, helper, ...", Required: true},
			"source":  schema.StringAttribute{Optional: true},
			"dest":    schema.StringAttribute{Optional: true},
			"proto":   schema.StringAttribute{Optional: true},
			"dport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"sport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"chain":   schema.StringAttribute{Optional: true},
			"helper":  schema.StringAttribute{Description: "Conntrack helper (ftp, sip, tftp, ...).", Optional: true},
			"user":    schema.StringAttribute{Optional: true},
			"comment": schema.StringAttribute{Optional: true},
		},
	}
}

func (r *conntrackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *conntrackResource) body(ctx context.Context, plan conntrackModel, diags *diag.Diagnostics) conntrackAPI {
	return conntrackAPI{
		Priority: int(plan.Priority.ValueInt64()),
		Action:   plan.Action.ValueString(),
		Source:   plan.Source.ValueString(),
		Dest:     plan.Dest.ValueString(),
		Proto:    plan.Proto.ValueString(),
		DPort:    listToStrings(ctx, plan.DPort, diags),
		SPort:    listToStrings(ctx, plan.SPort, diags),
		Chain:    plan.Chain.ValueString(),
		Helper:   plan.Helper.ValueString(),
		User:     plan.User.ValueString(),
		Comment:  plan.Comment.ValueString(),
	}
}

func (r *conntrackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan conntrackModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out conntrackAPI
	if err := r.client.post(ctx, "/api/v1/conntrack", body, &out); err != nil {
		resp.Diagnostics.AddError("create conntrack failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *conntrackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state conntrackModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out conntrackAPI
	if err := r.client.post(ctx, "/api/v1/conntrack", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate conntrack failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/conntrack/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old conntrack failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *conntrackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state conntrackModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out conntrackAPI
	if err := r.client.get(ctx, "/api/v1/conntrack/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read conntrack failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *conntrackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state conntrackModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/conntrack/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete conntrack failed", err.Error())
	}
}

func (r *conntrackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "conntrack id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *conntrackResource) toModel(ctx context.Context, api conntrackAPI, prior conntrackModel, diags *diag.Diagnostics) conntrackModel {
	return conntrackModel{
		ID:       types.Int64Value(api.ID),
		Priority: types.Int64Value(int64(api.Priority)),
		Action:   types.StringValue(api.Action),
		Source:   optionalString(api.Source, prior.Source),
		Dest:     optionalString(api.Dest, prior.Dest),
		Proto:    optionalString(api.Proto, prior.Proto),
		DPort:    optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort:    optionalList(ctx, api.SPort, prior.SPort, diags),
		Chain:    optionalString(api.Chain, prior.Chain),
		Helper:   optionalString(api.Helper, prior.Helper),
		User:     optionalString(api.User, prior.User),
		Comment:  optionalString(api.Comment, prior.Comment),
	}
}
