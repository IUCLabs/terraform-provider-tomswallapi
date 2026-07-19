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
	_ resource.Resource                = &deviceResource{}
	_ resource.ResourceWithImportState = &deviceResource{}
)

type deviceResource struct{ client *apiClient }

type deviceModel struct {
	Name     types.String `tfsdk:"name"`
	Class    types.String `tfsdk:"class"`
	Fabric   types.String `tfsdk:"fabric"`
	Resolver types.List   `tfsdk:"resolver"`
	Settings types.Map    `tfsdk:"settings"`
}

type deviceAPI struct {
	Name     string            `json:"name"`
	Class    string            `json:"class"`
	Fabric   string            `json:"fabric,omitempty"`
	Resolver []string          `json:"resolver,omitempty"`
	Settings map[string]string `json:"settings,omitempty"`
}

func NewDeviceResource() resource.Resource { return &deviceResource{} }

func (r *deviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (r *deviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A fleet member. Its class (router/firewall) and fabric determine how it enforces intents; bindings map global zones to its local interfaces.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Device name (globally unique).",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"class": schema.StringAttribute{
				Description: "Device class: router or firewall.",
				Required:    true,
			},
			"fabric": schema.StringAttribute{
				Description: "Routing domain (fabric) this device belongs to.",
				Optional:    true,
			},
			"resolver": schema.ListAttribute{
				Description: "Per-device DNS resolver override for dns address groups. Falls back to the fleet default, then system resolvers.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"settings": schema.MapAttribute{
				Description: "Per-device settings overrides (e.g. table_name, log_level, address_family).",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *deviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *deviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *deviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan deviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, plan, &resp.Diagnostics, &resp.State)
}

func (r *deviceResource) upsert(ctx context.Context, plan deviceModel, diags *diag.Diagnostics, state *tfsdk.State) {
	body := deviceAPI{
		Name:     plan.Name.ValueString(),
		Class:    plan.Class.ValueString(),
		Fabric:   plan.Fabric.ValueString(),
		Resolver: listToStrings(ctx, plan.Resolver, diags),
		Settings: mapToStrings(ctx, plan.Settings, diags),
	}
	if diags.HasError() {
		return
	}
	var out deviceAPI
	if err := r.client.put(ctx, "/api/v1/devices/"+pathEscape(body.Name), body, &out); err != nil {
		diags.AddError("upsert device failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, r.toModel(ctx, out, plan, diags))...)
}

func (r *deviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out deviceAPI
	if err := r.client.get(ctx, "/api/v1/devices/"+pathEscape(state.Name.ValueString()), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read device failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *deviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/devices/"+pathEscape(state.Name.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete device failed", err.Error())
	}
}

func (r *deviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *deviceResource) toModel(ctx context.Context, api deviceAPI, prior deviceModel, diags *diag.Diagnostics) deviceModel {
	m := deviceModel{
		Name:   types.StringValue(api.Name),
		Class:  types.StringValue(api.Class),
		Fabric: optionalString(api.Fabric, prior.Fabric),
	}
	// Preserve null for optional collections the user left unset.
	if prior.Resolver.IsNull() && len(api.Resolver) == 0 {
		m.Resolver = types.ListNull(types.StringType)
	} else {
		m.Resolver = stringsToList(ctx, api.Resolver, diags)
	}
	if prior.Settings.IsNull() && len(api.Settings) == 0 {
		m.Settings = types.MapNull(types.StringType)
	} else {
		m.Settings = stringsToMap(ctx, api.Settings, diags)
	}
	return m
}
