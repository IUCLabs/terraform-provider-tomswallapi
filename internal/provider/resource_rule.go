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
	_ resource.Resource                = &ruleResource{}
	_ resource.ResourceWithImportState = &ruleResource{}
)

type ruleResource struct{ client *apiClient }

type ruleModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Priority  types.Int64  `tfsdk:"priority"`
	Action    types.String `tfsdk:"action"`
	Source    types.List   `tfsdk:"source"`
	Dest      types.List   `tfsdk:"dest"`
	Proto     types.String `tfsdk:"proto"`
	PortGroup types.String `tfsdk:"portgroup"`
	Ports     types.List   `tfsdk:"ports"`
	Log       types.String `tfsdk:"log"`
	Comment   types.String `tfsdk:"comment"`
}

type ruleAPI struct {
	ID        int64    `json:"id,omitempty"`
	Priority  int      `json:"priority"`
	Action    string   `json:"action"`
	Source    []string `json:"source"`
	Dest      []string `json:"dest"`
	Proto     string   `json:"proto,omitempty"`
	PortGroup string   `json:"portgroup,omitempty"`
	Ports     []string `json:"ports,omitempty"`
	Log       string   `json:"log,omitempty"`
	Comment   string   `json:"comment,omitempty"`
}

func NewRuleResource() resource.Resource { return &ruleResource{} }

func (r *ruleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

func (r *ruleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A fleet-global firewall intent. Source and dest are shorewall-style element lists (bare zone, or zone:+ipset / zone:&fqdn); a selector must always be paired with a zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description:   "Server-assigned rule id.",
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"priority": schema.Int64Attribute{
				Description: "Evaluation priority; lower is evaluated first.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
			},
			"action": schema.StringAttribute{
				Description: "accept, drop, reject, etc.",
				Required:    true,
			},
			"source": schema.ListAttribute{
				Description: "Source element list, e.g. [\"loc\", \"net:+asn_cloudflare\"].",
				Required:    true,
				ElementType: types.StringType,
			},
			"dest": schema.ListAttribute{
				Description: "Dest element list.",
				Required:    true,
				ElementType: types.StringType,
			},
			"proto": schema.StringAttribute{
				Description: "Protocol (used when portgroup is not set).",
				Optional:    true,
			},
			"portgroup": schema.StringAttribute{
				Description: "Named portgroup supplying proto+ports.",
				Optional:    true,
			},
			"ports": schema.ListAttribute{
				Description: "Explicit ports (used when portgroup is not set).",
				Optional:    true,
				ElementType: types.StringType,
			},
			"log":     schema.StringAttribute{Description: "Log level for matching packets (e.g. info).", Optional: true},
			"comment": schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *ruleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *ruleResource) body(ctx context.Context, plan ruleModel, diags *diag.Diagnostics) ruleAPI {
	return ruleAPI{
		Priority:  int(plan.Priority.ValueInt64()),
		Action:    plan.Action.ValueString(),
		Source:    listToStrings(ctx, plan.Source, diags),
		Dest:      listToStrings(ctx, plan.Dest, diags),
		Proto:     plan.Proto.ValueString(),
		PortGroup: plan.PortGroup.ValueString(),
		Ports:     listToStrings(ctx, plan.Ports, diags),
		Log:       plan.Log.ValueString(),
		Comment:   plan.Comment.ValueString(),
	}
}

func (r *ruleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ruleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out ruleAPI
	if err := r.client.post(ctx, "/api/v1/rules", body, &out); err != nil {
		resp.Diagnostics.AddError("create rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

// Update: the rules API is create/delete only (no in-place PUT), so replace by
// deleting the old id and creating the new definition, keeping it in one apply.
func (r *ruleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ruleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out ruleAPI
	if err := r.client.post(ctx, "/api/v1/rules", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate rule failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/rules/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old rule failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *ruleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ruleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out ruleAPI
	if err := r.client.get(ctx, "/api/v1/rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *ruleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ruleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete rule failed", err.Error())
	}
}

func (r *ruleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "rule id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *ruleResource) toModel(ctx context.Context, api ruleAPI, prior ruleModel, diags *diag.Diagnostics) ruleModel {
	m := ruleModel{
		ID:        types.Int64Value(api.ID),
		Priority:  types.Int64Value(int64(api.Priority)),
		Action:    types.StringValue(api.Action),
		Source:    stringsToList(ctx, api.Source, diags),
		Dest:      stringsToList(ctx, api.Dest, diags),
		Proto:     optionalString(api.Proto, prior.Proto),
		PortGroup: optionalString(api.PortGroup, prior.PortGroup),
		Log:       optionalString(api.Log, prior.Log),
		Comment:   optionalString(api.Comment, prior.Comment),
	}
	// ports is optional; preserve null when unset and the API returns none.
	if prior.Ports.IsNull() && len(api.Ports) == 0 {
		m.Ports = types.ListNull(types.StringType)
	} else {
		m.Ports = stringsToList(ctx, api.Ports, diags)
	}
	return m
}
