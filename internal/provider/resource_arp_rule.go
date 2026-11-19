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
	_ resource.Resource                = &arpRuleResource{}
	_ resource.ResourceWithImportState = &arpRuleResource{}
)

type arpRuleResource struct{ client *apiClient }

type arpRuleModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Device        types.String `tfsdk:"device"`
	Action        types.String `tfsdk:"action"`
	ActionAddress types.String `tfsdk:"action_address"`
	ActionMAC     types.String `tfsdk:"action_mac"`
	Source        types.String `tfsdk:"source"`
	Dest          types.String `tfsdk:"dest"`
	Opcode        types.Int64  `tfsdk:"opcode"`
	Comment       types.String `tfsdk:"comment"`
}

type arpRuleAPI struct {
	ID            int64  `json:"id,omitempty"`
	Device        string `json:"device"`
	Action        string `json:"action"`
	ActionAddress string `json:"action_address,omitempty"`
	ActionMAC     string `json:"action_mac,omitempty"`
	Source        string `json:"source,omitempty"`
	Dest          string `json:"dest,omitempty"`
	Opcode        int    `json:"opcode,omitempty"`
	Comment       string `json:"comment,omitempty"`
}

func NewArpRuleResource() resource.Resource { return &arpRuleResource{} }

func (r *arpRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_arp_rule"
}

func (r *arpRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An ARP-level rule on a device.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"device":         schema.StringAttribute{Description: "Owning device.", Required: true},
			"action":         schema.StringAttribute{Description: "ARP action (ACCEPT, DROP, SNAT, or DNAT).", Required: true},
			"action_address": schema.StringAttribute{Description: "Replacement IP address for SNAT/DNAT actions.", Optional: true},
			"action_mac":     schema.StringAttribute{Description: "Replacement MAC address for SNAT/DNAT actions.", Optional: true},
			"source":         schema.StringAttribute{Description: "Source interface, address, or MAC to match.", Optional: true},
			"dest":           schema.StringAttribute{Description: "Destination interface, address, or MAC to match.", Optional: true},
			"opcode":         schema.Int64Attribute{Description: "ARP opcode to match (1 = request, 2 = reply).", Optional: true},
			"comment":        schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *arpRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *arpRuleResource) body(plan arpRuleModel) arpRuleAPI {
	return arpRuleAPI{
		Device:        plan.Device.ValueString(),
		Action:        plan.Action.ValueString(),
		ActionAddress: plan.ActionAddress.ValueString(),
		ActionMAC:     plan.ActionMAC.ValueString(),
		Source:        plan.Source.ValueString(),
		Dest:          plan.Dest.ValueString(),
		Opcode:        int(plan.Opcode.ValueInt64()),
		Comment:       plan.Comment.ValueString(),
	}
}

func (r *arpRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan arpRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out arpRuleAPI
	if err := r.client.post(ctx, "/api/v1/arp-rules", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create arp_rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *arpRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state arpRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out arpRuleAPI
	if err := r.client.post(ctx, "/api/v1/arp-rules", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate arp_rule failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/arp-rules/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old arp_rule failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *arpRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state arpRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out arpRuleAPI
	if err := r.client.get(ctx, "/api/v1/arp-rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read arp_rule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *arpRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state arpRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/arp-rules/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete arp_rule failed", err.Error())
	}
}

func (r *arpRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "arp_rule id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *arpRuleResource) toModel(api arpRuleAPI, prior arpRuleModel) arpRuleModel {
	return arpRuleModel{
		ID:            types.Int64Value(api.ID),
		Device:        types.StringValue(api.Device),
		Action:        types.StringValue(api.Action),
		ActionAddress: optionalString(api.ActionAddress, prior.ActionAddress),
		ActionMAC:     optionalString(api.ActionMAC, prior.ActionMAC),
		Source:        optionalString(api.Source, prior.Source),
		Dest:          optionalString(api.Dest, prior.Dest),
		Opcode:        optionalInt64(api.Opcode, prior.Opcode),
		Comment:       optionalString(api.Comment, prior.Comment),
	}
}
