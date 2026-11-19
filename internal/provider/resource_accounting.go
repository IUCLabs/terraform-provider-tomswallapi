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
	_ resource.Resource                = &accountingResource{}
	_ resource.ResourceWithImportState = &accountingResource{}
)

type accountingResource struct{ client *apiClient }

type accountingModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Device  types.String `tfsdk:"device"`
	Action  types.String `tfsdk:"action"`
	Section types.String `tfsdk:"section"`
	Chain   types.String `tfsdk:"chain"`
	Source  types.String `tfsdk:"source"`
	Dest    types.String `tfsdk:"dest"`
	Proto   types.String `tfsdk:"proto"`
	DPort   types.List   `tfsdk:"dport"`
	SPort   types.List   `tfsdk:"sport"`
	Mark    types.String `tfsdk:"mark"`
	Comment types.String `tfsdk:"comment"`
}

type accountingAPI struct {
	ID      int64    `json:"id,omitempty"`
	Device  string   `json:"device"`
	Action  string   `json:"action"`
	Section string   `json:"section,omitempty"`
	Chain   string   `json:"chain,omitempty"`
	Source  string   `json:"source,omitempty"`
	Dest    string   `json:"dest,omitempty"`
	Proto   string   `json:"proto,omitempty"`
	DPort   []string `json:"dport,omitempty"`
	SPort   []string `json:"sport,omitempty"`
	Mark    string   `json:"mark,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

func NewAccountingResource() resource.Resource { return &accountingResource{} }

func (r *accountingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_accounting"
}

func (r *accountingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A traffic-accounting rule on a device.",
		Attributes: map[string]schema.Attribute{
			"id":      schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":  schema.StringAttribute{Description: "Owning device.", Required: true},
			"action":  schema.StringAttribute{Description: "Accounting action (COUNT, DONE, or a chain name).", Required: true},
			"section": schema.StringAttribute{Description: "Accounting section (INPUT, OUTPUT, or FORWARD).", Optional: true},
			"chain":   schema.StringAttribute{Description: "Accounting chain the rule is added to.", Optional: true},
			"source":  schema.StringAttribute{Description: "Source to match (zone, interface, or address in shorewall notation).", Optional: true},
			"dest":    schema.StringAttribute{Description: "Destination to match (zone, interface, or address in shorewall notation).", Optional: true},
			"proto":   schema.StringAttribute{Description: "Protocol to match (tcp, udp, icmp, ...).", Optional: true},
			"dport":   schema.ListAttribute{Description: "Destination ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"sport":   schema.ListAttribute{Description: "Source ports or port ranges to match.", Optional: true, ElementType: types.StringType},
			"mark":    schema.StringAttribute{Description: "Packet mark to match (value[/mask]).", Optional: true},
			"comment": schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *accountingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *accountingResource) body(ctx context.Context, plan accountingModel, diags *diag.Diagnostics) accountingAPI {
	return accountingAPI{
		Device: plan.Device.ValueString(), Action: plan.Action.ValueString(), Section: plan.Section.ValueString(),
		Chain: plan.Chain.ValueString(), Source: plan.Source.ValueString(), Dest: plan.Dest.ValueString(),
		Proto: plan.Proto.ValueString(), DPort: listToStrings(ctx, plan.DPort, diags), SPort: listToStrings(ctx, plan.SPort, diags),
		Mark: plan.Mark.ValueString(), Comment: plan.Comment.ValueString(),
	}
}

func (r *accountingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out accountingAPI
	if err := r.client.post(ctx, "/api/v1/accounting", body, &out); err != nil {
		resp.Diagnostics.AddError("create accounting failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *accountingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accountingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out accountingAPI
	if err := r.client.post(ctx, "/api/v1/accounting", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate accounting failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/accounting/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old accounting failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *accountingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out accountingAPI
	if err := r.client.get(ctx, "/api/v1/accounting/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read accounting failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *accountingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accountingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/accounting/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete accounting failed", err.Error())
	}
}

func (r *accountingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "accounting id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *accountingResource) toModel(ctx context.Context, api accountingAPI, prior accountingModel, diags *diag.Diagnostics) accountingModel {
	return accountingModel{
		ID: types.Int64Value(api.ID), Device: types.StringValue(api.Device), Action: types.StringValue(api.Action),
		Section: optionalString(api.Section, prior.Section), Chain: optionalString(api.Chain, prior.Chain),
		Source: optionalString(api.Source, prior.Source), Dest: optionalString(api.Dest, prior.Dest),
		Proto: optionalString(api.Proto, prior.Proto), DPort: optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort: optionalList(ctx, api.SPort, prior.SPort, diags), Mark: optionalString(api.Mark, prior.Mark),
		Comment: optionalString(api.Comment, prior.Comment),
	}
}
