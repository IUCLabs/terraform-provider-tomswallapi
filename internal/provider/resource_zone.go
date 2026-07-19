package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &zoneResource{}
	_ resource.ResourceWithImportState = &zoneResource{}
)

type zoneResource struct{ client *apiClient }

type zoneModel struct {
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Subnets types.List   `tfsdk:"subnets"`
	Parent  types.String `tfsdk:"parent"`
}

type zoneAPI struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Subnets []string `json:"subnets"`
	Parent  string   `json:"parent,omitempty"`
}

func NewZoneResource() resource.Resource { return &zoneResource{} }

func (r *zoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *zoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A fleet-global network segment (a named set of subnets). Zones are the topological anchors of the firewall model.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Zone name (globally unique).",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Description: "Zone type: ip, ip6, or firewall.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("ip"),
			},
			"subnets": schema.ListAttribute{
				Description: "CIDRs belonging to this zone. May be empty for a no-subnet zone such as an internet-facing 'net'.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"parent": schema.StringAttribute{
				Description: "Parent zone name for subzone nesting; the child's subnets must be within the parent's.",
				Optional:    true,
			},
		},
	}
}

func (r *zoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *zoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *zoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan zoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *zoneResource) upsert(ctx context.Context, plan zoneModel, diags *diag.Diagnostics, state *tfsdk.State) {
	body := zoneAPI{
		Name:    plan.Name.ValueString(),
		Type:    plan.Type.ValueString(),
		Subnets: listToStrings(ctx, plan.Subnets, diags),
		Parent:  plan.Parent.ValueString(),
	}
	if diags.HasError() {
		return
	}
	var out zoneAPI
	if err := r.client.put(ctx, "/api/v1/zones/"+pathEscape(body.Name), body, &out); err != nil {
		diags.AddError("upsert zone failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, r.toModel(ctx, out, diags))...)
}

func (r *zoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out zoneAPI
	if err := r.client.get(ctx, "/api/v1/zones/"+pathEscape(state.Name.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read zone failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, &resp.Diagnostics))...)
}

func (r *zoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/zones/"+pathEscape(state.Name.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete zone failed", err.Error())
	}
}

func (r *zoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *zoneResource) toModel(ctx context.Context, api zoneAPI, diags *diag.Diagnostics) zoneModel {
	m := zoneModel{
		Name:    types.StringValue(api.Name),
		Type:    types.StringValue(api.Type),
		Subnets: stringsToList(ctx, api.Subnets, diags),
	}
	if api.Parent == "" {
		m.Parent = types.StringNull()
	} else {
		m.Parent = types.StringValue(api.Parent)
	}
	return m
}
