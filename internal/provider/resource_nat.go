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
	_ resource.Resource                = &natResource{}
	_ resource.ResourceWithImportState = &natResource{}
)

type natResource struct{ client *apiClient }

type natModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Device    types.String `tfsdk:"device"`
	External  types.String `tfsdk:"external"`
	Internal  types.String `tfsdk:"internal"`
	Interface types.String `tfsdk:"interface"`
	Comment   types.String `tfsdk:"comment"`
}

type natAPI struct {
	ID        int64  `json:"id,omitempty"`
	Device    string `json:"device"`
	External  string `json:"external"`
	Internal  string `json:"internal"`
	Interface string `json:"interface,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

func NewNATResource() resource.Resource { return &natResource{} }

func (r *natResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nat"
}

func (r *natResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A one-to-one static NAT bound to the device that owns the external IP.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description:   "Server-assigned id.",
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"device": schema.StringAttribute{
				Description: "Device that holds the external address.",
				Required:    true,
			},
			"external": schema.StringAttribute{
				Description: "External IP address.",
				Required:    true,
			},
			"internal": schema.StringAttribute{
				Description: "Internal IP address.",
				Required:    true,
			},
			"interface": schema.StringAttribute{
				Description: "Interface that has the external address.",
				Optional:    true,
			},
			"comment": schema.StringAttribute{Description: "Free-form comment.", Optional: true},
		},
	}
}

func (r *natResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *natResource) body(plan natModel) natAPI {
	return natAPI{
		Device:    plan.Device.ValueString(),
		External:  plan.External.ValueString(),
		Internal:  plan.Internal.ValueString(),
		Interface: plan.Interface.ValueString(),
		Comment:   plan.Comment.ValueString(),
	}
}

func (r *natResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan natModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out natAPI
	if err := r.client.post(ctx, "/api/v1/nat", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("create nat failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

// Update: the nat API is create/delete only, so recreate and delete the old id.
func (r *natResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state natModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out natAPI
	if err := r.client.post(ctx, "/api/v1/nat", r.body(plan), &out); err != nil {
		resp.Diagnostics.AddError("recreate nat failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/nat/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old nat failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, plan))...)
}

func (r *natResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state natModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out natAPI
	if err := r.client.get(ctx, "/api/v1/nat/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read nat failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(out, state))...)
}

func (r *natResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state natModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/nat/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete nat failed", err.Error())
	}
}

func (r *natResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "nat id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *natResource) toModel(api natAPI, prior natModel) natModel {
	return natModel{
		ID:        types.Int64Value(api.ID),
		Device:    types.StringValue(api.Device),
		External:  types.StringValue(api.External),
		Internal:  types.StringValue(api.Internal),
		Interface: optionalString(api.Interface, prior.Interface),
		Comment:   optionalString(api.Comment, prior.Comment),
	}
}
