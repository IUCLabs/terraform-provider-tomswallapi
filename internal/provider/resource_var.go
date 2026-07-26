package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &varResource{}
	_ resource.ResourceWithImportState = &varResource{}
)

type varResource struct{ client *apiClient }

type varModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type varAPI struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewVarResource() resource.Resource { return &varResource{} }

func (r *varResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_var"
}

func (r *varResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A global substitution variable (key/value) usable across the fleet config.",
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Description:   "Variable name.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{Required: true},
		},
	}
}

func (r *varResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *varResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan varModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out varAPI
	if err := r.client.put(ctx, "/api/v1/vars/"+pathEscape(plan.Key.ValueString()), varAPI{Key: plan.Key.ValueString(), Value: plan.Value.ValueString()}, &out); err != nil {
		resp.Diagnostics.AddError("upsert var failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, varModel{Key: types.StringValue(out.Key), Value: types.StringValue(out.Value)})...)
}

func (r *varResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan varModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out varAPI
	if err := r.client.put(ctx, "/api/v1/vars/"+pathEscape(plan.Key.ValueString()), varAPI{Key: plan.Key.ValueString(), Value: plan.Value.ValueString()}, &out); err != nil {
		resp.Diagnostics.AddError("upsert var failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, varModel{Key: types.StringValue(out.Key), Value: types.StringValue(out.Value)})...)
}

func (r *varResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state varModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out varAPI
	if err := r.client.get(ctx, "/api/v1/vars/"+pathEscape(state.Key.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read var failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, varModel{Key: types.StringValue(out.Key), Value: types.StringValue(out.Value)})...)
}

func (r *varResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state varModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/vars/"+pathEscape(state.Key.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete var failed", err.Error())
	}
}

func (r *varResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
