package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &fabricResource{}
	_ resource.ResourceWithImportState = &fabricResource{}
)

type fabricResource struct{ client *apiClient }

type fabricModel struct {
	Name             types.String `tfsdk:"name"`
	EnforceOnRouters types.Bool   `tfsdk:"enforce_on_routers"`
	Description      types.String `tfsdk:"description"`
}

type fabricAPI struct {
	Name             string `json:"name"`
	EnforceOnRouters bool   `json:"enforce_on_routers"`
	Description      string `json:"description,omitempty"`
}

func NewFabricResource() resource.Resource { return &fabricResource{} }

func (r *fabricResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fabric"
}

func (r *fabricResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A routing domain. enforce_on_routers toggles defense-in-depth (every router carries the intent) versus transparent transit (only boundary firewalls enforce).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Fabric name (globally unique).",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enforce_on_routers": schema.BoolAttribute{
				Description: "When true, routers in this fabric enforce intents too (defense-in-depth).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
		},
	}
}

func (r *fabricResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *fabricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan fabricModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *fabricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan fabricModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *fabricResource) upsert(ctx context.Context, plan fabricModel, diags *diag.Diagnostics, state *tfsdk.State) {
	body := fabricAPI{
		Name:             plan.Name.ValueString(),
		EnforceOnRouters: plan.EnforceOnRouters.ValueBool(),
		Description:      plan.Description.ValueString(),
	}
	var out fabricAPI
	if err := r.client.put(ctx, "/api/v1/fabrics/"+pathEscape(body.Name), body, &out); err != nil {
		diags.AddError("upsert fabric failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, fabricToModel(out))...)
}

func (r *fabricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state fabricModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out fabricAPI
	if err := r.client.get(ctx, "/api/v1/fabrics/"+pathEscape(state.Name.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read fabric failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fabricToModel(out))...)
}

func (r *fabricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state fabricModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/fabrics/"+pathEscape(state.Name.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete fabric failed", err.Error())
	}
}

func (r *fabricResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func fabricToModel(api fabricAPI) fabricModel {
	return fabricModel{
		Name:             types.StringValue(api.Name),
		EnforceOnRouters: types.BoolValue(api.EnforceOnRouters),
		Description:      types.StringValue(api.Description),
	}
}
