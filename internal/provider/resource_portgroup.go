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
	_ resource.Resource                = &portGroupResource{}
	_ resource.ResourceWithImportState = &portGroupResource{}
)

type portGroupResource struct{ client *apiClient }

type portGroupModel struct {
	Name  types.String `tfsdk:"name"`
	Proto types.String `tfsdk:"proto"`
	Ports types.List   `tfsdk:"ports"`
}

type portGroupAPI struct {
	Name  string   `json:"name"`
	Proto string   `json:"proto"`
	Ports []string `json:"ports"`
}

func NewPortGroupResource() resource.Resource { return &portGroupResource{} }

func (r *portGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_portgroup"
}

func (r *portGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A reusable proto+ports combination referenced by rules.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Portgroup name (globally unique).",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"proto": schema.StringAttribute{
				Description: "Protocol, e.g. tcp or udp.",
				Required:    true,
			},
			"ports": schema.ListAttribute{
				Description: "Ports or port ranges, e.g. [\"80\", \"443\", \"1024-65535\"].",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *portGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *portGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *portGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *portGroupResource) upsert(ctx context.Context, plan portGroupModel, diags *diag.Diagnostics, state *tfsdk.State) {
	body := portGroupAPI{
		Name:  plan.Name.ValueString(),
		Proto: plan.Proto.ValueString(),
		Ports: listToStrings(ctx, plan.Ports, diags),
	}
	if diags.HasError() {
		return
	}
	var out portGroupAPI
	if err := r.client.put(ctx, "/api/v1/portgroups/"+pathEscape(body.Name), body, &out); err != nil {
		diags.AddError("upsert portgroup failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, r.toModel(ctx, out, diags))...)
}

func (r *portGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out portGroupAPI
	if err := r.client.get(ctx, "/api/v1/portgroups/"+pathEscape(state.Name.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read portgroup failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, &resp.Diagnostics))...)
}

func (r *portGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state portGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/portgroups/"+pathEscape(state.Name.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete portgroup failed", err.Error())
	}
}

func (r *portGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *portGroupResource) toModel(ctx context.Context, api portGroupAPI, diags *diag.Diagnostics) portGroupModel {
	return portGroupModel{
		Name:  types.StringValue(api.Name),
		Proto: types.StringValue(api.Proto),
		Ports: stringsToList(ctx, api.Ports, diags),
	}
}
