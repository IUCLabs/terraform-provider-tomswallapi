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
	_ resource.Resource                = &snatResource{}
	_ resource.ResourceWithImportState = &snatResource{}
)

type snatResource struct{ client *apiClient }

type snatModel struct {
	ID          types.Int64   `tfsdk:"id"`
	Action      types.String  `tfsdk:"action"`
	Source      types.String  `tfsdk:"source"`
	Egress      types.String  `tfsdk:"egress"`
	Address     types.String  `tfsdk:"address"`
	Probability types.Float64 `tfsdk:"probability"`
	Comment     types.String  `tfsdk:"comment"`
}

type snatAPI struct {
	ID          int64    `json:"id,omitempty"`
	Action      string   `json:"action"`
	Source      string   `json:"source"`
	Egress      string   `json:"egress"`
	Address     string   `json:"address,omitempty"`
	Probability *float64 `json:"probability,omitempty"`
	Comment     string   `json:"comment,omitempty"`
}

func NewSNATResource() resource.Resource { return &snatResource{} }

func (r *snatResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snat"
}

func (r *snatResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A source-NAT / masquerade intent. It renders on devices that bind the egress zone (and, for a zone source, that zone too) — auto-scoping masquerade to edges.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description:   "Server-assigned id.",
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"action": schema.StringAttribute{
				Description: "masquerade or snat.",
				Required:    true,
			},
			"source": schema.StringAttribute{
				Description: "Source zone name or a literal CIDR to masquerade/SNAT.",
				Required:    true,
			},
			"egress": schema.StringAttribute{
				Description: "Egress zone; resolved per device to its egress interface via bindings.",
				Required:    true,
			},
			"address": schema.StringAttribute{
				Description: "For snat: the address (or range) to rewrite the source to.",
				Optional:    true,
			},
			"probability": schema.Float64Attribute{
				Description: "Match probability (0-1) for load-balanced SNAT.",
				Optional:    true,
			},
			"comment": schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *snatResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *snatResource) body(plan snatModel) snatAPI {
	b := snatAPI{
		Action:  plan.Action.ValueString(),
		Source:  plan.Source.ValueString(),
		Egress:  plan.Egress.ValueString(),
		Address: plan.Address.ValueString(),
		Comment: plan.Comment.ValueString(),
	}
	if !plan.Probability.IsNull() && !plan.Probability.IsUnknown() {
		p := plan.Probability.ValueFloat64()
		b.Probability = &p
	}
	return b
}

func (r *snatResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan snatModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out snatAPI
	if err := r.client.post(ctx, "/api/v1/snat", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create snat failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

// Update: the snat API is create/delete only, so recreate and delete the old id.
func (r *snatResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state snatModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out snatAPI
	if err := r.client.post(ctx, "/api/v1/snat", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate snat failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/snat/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old snat failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *snatResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state snatModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out snatAPI
	if err := r.client.get(ctx, "/api/v1/snat/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read snat failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *snatResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state snatModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/snat/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete snat failed", err.Error())
	}
}

func (r *snatResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "snat id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *snatResource) toModel(api snatAPI, prior snatModel) snatModel {
	m := snatModel{
		ID:      types.Int64Value(api.ID),
		Action:  types.StringValue(api.Action),
		Source:  types.StringValue(api.Source),
		Egress:  types.StringValue(api.Egress),
		Address: optionalString(api.Address, prior.Address),
		Comment: optionalString(api.Comment, prior.Comment),
	}
	if api.Probability != nil {
		m.Probability = types.Float64Value(*api.Probability)
	} else {
		m.Probability = prior.Probability
	}
	return m
}
