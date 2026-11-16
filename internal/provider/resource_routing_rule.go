package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &routingRuleResource{}
	_ resource.ResourceWithImportState = &routingRuleResource{}
)

type routingRuleResource struct{ client *apiClient }

type routingRuleModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Device     types.String `tfsdk:"device"`
	Source     types.String `tfsdk:"source"`
	Dest       types.String `tfsdk:"dest"`
	Provider   types.String `tfsdk:"routing_provider"`
	Priority   types.Int64  `tfsdk:"priority"`
	Persistent types.Bool   `tfsdk:"persistent"`
	Mark       types.String `tfsdk:"mark"`
	Comment    types.String `tfsdk:"comment"`
}

type routingRuleAPI struct {
	ID         int64  `json:"id,omitempty"`
	Device     string `json:"device"`
	Source     string `json:"source,omitempty"`
	Dest       string `json:"dest,omitempty"`
	Provider   string `json:"provider"`
	Priority   int    `json:"priority,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
	Mark       string `json:"mark,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

func NewRoutingRuleResource() resource.Resource { return &routingRuleResource{} }

func (r *routingRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routing_rule"
}

func (r *routingRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Directs traffic to a provider's routing table on a device (rtrules).",
		Attributes: map[string]schema.Attribute{
			"id":               schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":           schema.StringAttribute{Description: "Owning device.", Required: true},
			"source":           schema.StringAttribute{Description: "Source address, network, or interface to match.", Optional: true},
			"dest":             schema.StringAttribute{Description: "Destination address or network to match.", Optional: true},
			"routing_provider": schema.StringAttribute{Description: "Provider whose routing table matching traffic uses.", Required: true},
			"priority":         schema.Int64Attribute{Description: "Routing rule priority; lower first.", Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
			"persistent":       schema.BoolAttribute{Description: "Keep the entry in place while the firewall is stopped.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"mark":             schema.StringAttribute{Description: "Firewall mark to match.", Optional: true},
			"comment":          schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *routingRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *routingRuleResource) body(plan routingRuleModel) routingRuleAPI {
	return routingRuleAPI{
		Device:     plan.Device.ValueString(),
		Source:     plan.Source.ValueString(),
		Dest:       plan.Dest.ValueString(),
		Provider:   plan.Provider.ValueString(),
		Priority:   int(plan.Priority.ValueInt64()),
		Persistent: plan.Persistent.ValueBool(),
		Mark:       plan.Mark.ValueString(),
		Comment:    plan.Comment.ValueString(),
	}
}

func (r *routingRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan routingRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out routingRuleAPI
	if err := r.client.post(ctx, "/api/v1/routing-rules", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create routing_rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *routingRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state routingRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out routingRuleAPI
	if err := r.client.post(ctx, "/api/v1/routing-rules", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate routing_rule failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/routing-rules/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old routing_rule failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *routingRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state routingRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out routingRuleAPI
	if err := r.client.get(ctx, "/api/v1/routing-rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read routing_rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *routingRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state routingRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/routing-rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete routing_rule failed", err.Error())
	}
}

func (r *routingRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "routing_rule id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *routingRuleResource) toModel(api routingRuleAPI, prior routingRuleModel) routingRuleModel {
	return routingRuleModel{
		ID:         types.Int64Value(api.ID),
		Device:     types.StringValue(api.Device),
		Source:     optionalString(api.Source, prior.Source),
		Dest:       optionalString(api.Dest, prior.Dest),
		Provider:   types.StringValue(api.Provider),
		Priority:   types.Int64Value(int64(api.Priority)),
		Persistent: types.BoolValue(api.Persistent),
		Mark:       optionalString(api.Mark, prior.Mark),
		Comment:    optionalString(api.Comment, prior.Comment),
	}
}
