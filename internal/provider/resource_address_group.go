package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &addressGroupResource{}
	_ resource.ResourceWithImportState = &addressGroupResource{}
)

type addressGroupResource struct{ client *apiClient }

type addressGroupModel struct {
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	Members     types.List   `tfsdk:"members"`
	Refresh     types.String `tfsdk:"refresh"`
	Description types.String `tfsdk:"description"`
	Resolved    types.List   `tfsdk:"resolved"`
	ResolvedAt  types.String `tfsdk:"resolved_at"`
}

type addressGroupAPI struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Members     []string `json:"members"`
	Refresh     string   `json:"refresh,omitempty"`
	Description string   `json:"description,omitempty"`
	Resolved    []string `json:"resolved,omitempty"`
	ResolvedAt  string   `json:"resolved_at,omitempty"`
}

func NewAddressGroupResource() resource.Resource { return &addressGroupResource{} }

func (r *addressGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_address_group"
}

func (r *addressGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An address group, materialized as an nftables named set. Population source is static (CIDRs), dns (FQDNs resolved on-device), or asn (ASNs expanded centrally).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Group name (globally unique). ASN groups conventionally use an asn_ prefix.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Description: "Population source: static, dns, or asn.",
				Required:    true,
			},
			"members": schema.ListAttribute{
				Description: "For static: CIDRs/IPs. For dns: FQDNs. For asn: ASN numbers.",
				Required:    true,
				ElementType: types.StringType,
			},
			"refresh": schema.StringAttribute{
				Description: "asn: prefix cache TTL (e.g. 24h). dns: honor_ttl.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Free-form description.",
				Optional:    true,
			},
			"resolved": schema.ListAttribute{
				Description: "Server-managed: concrete prefixes the ASN expander last produced.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"resolved_at": schema.StringAttribute{
				Description: "Server-managed: timestamp of the last ASN expansion.",
				Computed:    true,
			},
		},
	}
}

func (r *addressGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *addressGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan addressGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *addressGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan addressGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *addressGroupResource) upsert(ctx context.Context, plan addressGroupModel, diags *diag.Diagnostics, state *tfsdk.State) {
	body := addressGroupAPI{
		Name:        plan.Name.ValueString(),
		Type:        plan.Type.ValueString(),
		Members:     listToStrings(ctx, plan.Members, diags),
		Refresh:     plan.Refresh.ValueString(),
		Description: plan.Description.ValueString(),
	}
	if diags.HasError() {
		return
	}
	var out addressGroupAPI
	if err := r.client.put(ctx, "/api/v1/address-groups/"+pathEscape(body.Name), body, &out); err != nil {
		diags.AddError("upsert address_group failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, r.toModel(ctx, out, plan, diags))...)
}

func (r *addressGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state addressGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out addressGroupAPI
	if err := r.client.get(ctx, "/api/v1/address-groups/"+pathEscape(state.Name.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read address_group failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *addressGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state addressGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/address-groups/"+pathEscape(state.Name.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete address_group failed", err.Error())
	}
}

func (r *addressGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// toModel maps the API response back to state. Optional string fields fall back
// to the prior plan/state value so that omitempty responses don't churn state.
func (r *addressGroupResource) toModel(ctx context.Context, api addressGroupAPI, prior addressGroupModel, diags *diag.Diagnostics) addressGroupModel {
	m := addressGroupModel{
		Name:        types.StringValue(api.Name),
		Type:        types.StringValue(api.Type),
		Members:     stringsToList(ctx, api.Members, diags),
		Refresh:     optionalString(api.Refresh, prior.Refresh),
		Description: optionalString(api.Description, prior.Description),
		Resolved:    stringsToList(ctx, api.Resolved, diags),
		ResolvedAt:  types.StringValue(api.ResolvedAt),
	}
	return m
}

// optionalString returns a value for an optional attribute: the API value when
// present, else the prior planned value (preserving null when both are empty).
func optionalString(apiVal string, prior types.String) types.String {
	if apiVal != "" {
		return types.StringValue(apiVal)
	}
	return prior
}
