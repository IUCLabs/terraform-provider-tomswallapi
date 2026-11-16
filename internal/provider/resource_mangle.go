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
	_ resource.Resource                = &mangleResource{}
	_ resource.ResourceWithImportState = &mangleResource{}
)

type mangleResource struct{ client *apiClient }

type mangleModel struct {
	ID          types.Int64   `tfsdk:"id"`
	Device      types.String  `tfsdk:"device"`
	Action      types.String  `tfsdk:"action"`
	Chain       types.String  `tfsdk:"chain"`
	MarkValue   types.String  `tfsdk:"mark_value"`
	Source      types.String  `tfsdk:"source"`
	Dest        types.String  `tfsdk:"dest"`
	Proto       types.String  `tfsdk:"proto"`
	DPort       types.List    `tfsdk:"dport"`
	SPort       types.List    `tfsdk:"sport"`
	User        types.String  `tfsdk:"user"`
	Mark        types.String  `tfsdk:"mark"`
	Length      types.String  `tfsdk:"length"`
	TOS         types.String  `tfsdk:"tos"`
	Helper      types.String  `tfsdk:"helper"`
	Probability types.Float64 `tfsdk:"probability"`
	Comment     types.String  `tfsdk:"comment"`
}

type mangleAPI struct {
	ID          int64    `json:"id,omitempty"`
	Device      string   `json:"device"`
	Action      string   `json:"action"`
	Chain       string   `json:"chain,omitempty"`
	MarkValue   string   `json:"mark_value,omitempty"`
	Source      string   `json:"source,omitempty"`
	Dest        string   `json:"dest,omitempty"`
	Proto       string   `json:"proto,omitempty"`
	DPort       []string `json:"dport,omitempty"`
	SPort       []string `json:"sport,omitempty"`
	User        string   `json:"user,omitempty"`
	Mark        string   `json:"mark,omitempty"`
	Length      string   `json:"length,omitempty"`
	TOS         string   `json:"tos,omitempty"`
	Helper      string   `json:"helper,omitempty"`
	Probability *float64 `json:"probability,omitempty"`
	Comment     string   `json:"comment,omitempty"`
}

func NewMangleResource() resource.Resource { return &mangleResource{} }

func (r *mangleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mangle"
}

func (r *mangleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A packet-mangling rule (marking, TOS, etc.) on a device.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":      schema.StringAttribute{Description: "Owning device.", Required: true},
			"action":      schema.StringAttribute{Description: "Mangle action (MARK, CONNMARK, TOS, ...).", Required: true},
			"chain":       schema.StringAttribute{Description: "Netfilter chain (e.g. PREROUTING, FORWARD).", Optional: true},
			"mark_value":  schema.StringAttribute{Description: "Value the action sets.", Optional: true},
			"source":      schema.StringAttribute{Description: "Source to match (zone, interface, or address in shorewall notation).", Optional: true},
			"dest":        schema.StringAttribute{Description: "Destination to match (zone, interface, or address in shorewall notation).", Optional: true},
			"proto":       schema.StringAttribute{Description: "Protocol to match (tcp, udp, icmp, ...).", Optional: true},
			"dport":       schema.ListAttribute{Description: "Destination ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"sport":       schema.ListAttribute{Description: "Source ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"user":        schema.StringAttribute{Description: "Owner of locally generated packets to match (user[:group]).", Optional: true},
			"mark":        schema.StringAttribute{Description: "Existing packet mark to match (value[/mask]).", Optional: true},
			"length":      schema.StringAttribute{Description: "Packet length or length range to match.", Optional: true},
			"tos":         schema.StringAttribute{Description: "Type of Service value to match.", Optional: true},
			"helper":      schema.StringAttribute{Description: "Conntrack helper the packets must be associated with.", Optional: true},
			"probability": schema.Float64Attribute{Description: "Fraction of matching packets the rule applies to (0 to 1).", Optional: true},
			"comment":     schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *mangleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *mangleResource) body(ctx context.Context, plan mangleModel, diags *diag.Diagnostics) mangleAPI {
	b := mangleAPI{
		Device: plan.Device.ValueString(), Action: plan.Action.ValueString(), Chain: plan.Chain.ValueString(),
		MarkValue: plan.MarkValue.ValueString(), Source: plan.Source.ValueString(), Dest: plan.Dest.ValueString(),
		Proto: plan.Proto.ValueString(), DPort: listToStrings(ctx, plan.DPort, diags), SPort: listToStrings(ctx, plan.SPort, diags),
		User: plan.User.ValueString(), Mark: plan.Mark.ValueString(), Length: plan.Length.ValueString(),
		TOS: plan.TOS.ValueString(), Helper: plan.Helper.ValueString(), Comment: plan.Comment.ValueString(),
	}
	if !plan.Probability.IsNull() && !plan.Probability.IsUnknown() {
		p := plan.Probability.ValueFloat64()
		b.Probability = &p
	}
	return b
}

func (r *mangleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mangleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out mangleAPI
	if err := r.client.post(ctx, "/api/v1/mangle", body, &out); err != nil {
		resp.Diagnostics.AddError("create mangle failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *mangleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state mangleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out mangleAPI
	if err := r.client.post(ctx, "/api/v1/mangle", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate mangle failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/mangle/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old mangle failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *mangleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mangleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out mangleAPI
	if err := r.client.get(ctx, "/api/v1/mangle/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read mangle failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *mangleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mangleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/mangle/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete mangle failed", err.Error())
	}
}

func (r *mangleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "mangle id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *mangleResource) toModel(ctx context.Context, api mangleAPI, prior mangleModel, diags *diag.Diagnostics) mangleModel {
	m := mangleModel{
		ID: types.Int64Value(api.ID), Device: types.StringValue(api.Device), Action: types.StringValue(api.Action),
		Chain: optionalString(api.Chain, prior.Chain), MarkValue: optionalString(api.MarkValue, prior.MarkValue),
		Source: optionalString(api.Source, prior.Source), Dest: optionalString(api.Dest, prior.Dest),
		Proto: optionalString(api.Proto, prior.Proto), DPort: optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort: optionalList(ctx, api.SPort, prior.SPort, diags), User: optionalString(api.User, prior.User),
		Mark: optionalString(api.Mark, prior.Mark), Length: optionalString(api.Length, prior.Length),
		TOS: optionalString(api.TOS, prior.TOS), Helper: optionalString(api.Helper, prior.Helper),
		Comment: optionalString(api.Comment, prior.Comment),
	}
	if api.Probability != nil {
		m.Probability = types.Float64Value(*api.Probability)
	} else {
		m.Probability = prior.Probability
	}
	return m
}
