package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                = &bindingResource{}
	_ resource.ResourceWithImportState = &bindingResource{}
)

type bindingResource struct{ client *apiClient }

type bindingModel struct {
	Device     types.String `tfsdk:"device"`
	Zone       types.String `tfsdk:"zone"`
	Interfaces types.List   `tfsdk:"interfaces"`
}

type bindingAPI struct {
	Device     string   `json:"device"`
	Zone       string   `json:"zone"`
	Interfaces []string `json:"interfaces"`
}

func NewBindingResource() resource.Resource { return &bindingResource{} }

func (r *bindingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_binding"
}

func (r *bindingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Binds a global zone to a device's local interface(s) — the only legitimately host-specific object in the model.",
		Attributes: map[string]schema.Attribute{
			"device": schema.StringAttribute{
				Description:   "Device name.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"zone": schema.StringAttribute{
				Description:   "Global zone name.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"interfaces": schema.ListAttribute{
				Description: "Local interface names this zone attaches to on the device, e.g. [\"eth1\"].",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *bindingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *bindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *bindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan bindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *bindingResource) bindingPath(device, zone string) string {
	return "/api/v1/devices/" + pathEscape(device) + "/bindings/" + pathEscape(zone)
}

func (r *bindingResource) upsert(ctx context.Context, plan bindingModel, diags *diag.Diagnostics, state *tfsdk.State) {
	body := bindingAPI{
		Device:     plan.Device.ValueString(),
		Zone:       plan.Zone.ValueString(),
		Interfaces: listToStrings(ctx, plan.Interfaces, diags),
	}
	if diags.HasError() {
		return
	}
	var out bindingAPI
	if err := r.client.put(ctx, r.bindingPath(body.Device, body.Zone), body, &out); err != nil {
		diags.AddError("upsert binding failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, r.toModel(ctx, out, diags))...)
}

func (r *bindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state bindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out bindingAPI
	if err := r.client.get(ctx, r.bindingPath(state.Device.ValueString(), state.Zone.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read binding failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, &resp.Diagnostics))...)
}

func (r *bindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, r.bindingPath(state.Device.ValueString(), state.Zone.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete binding failed", err.Error())
	}
}

// ImportState accepts "device:zone".
func (r *bindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	device, zone, ok := strings.Cut(req.ID, ":")
	if !ok || device == "" || zone == "" {
		resp.Diagnostics.AddError("invalid import ID", fmt.Sprintf("expected device:zone, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("device"), device)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
}

func (r *bindingResource) toModel(ctx context.Context, api bindingAPI, diags *diag.Diagnostics) bindingModel {
	return bindingModel{
		Device:     types.StringValue(api.Device),
		Zone:       types.StringValue(api.Zone),
		Interfaces: stringsToList(ctx, api.Interfaces, diags),
	}
}
