package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secmarkResource{}
	_ resource.ResourceWithImportState = &secmarkResource{}
)

type secmarkResource struct{ client *apiClient }

type secmarkModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Secmark types.String `tfsdk:"secmark"`
	Chain   types.String `tfsdk:"chain"`
	Source  types.String `tfsdk:"source"`
	Dest    types.String `tfsdk:"dest"`
	Proto   types.String `tfsdk:"proto"`
	DPort   types.List   `tfsdk:"dport"`
	SPort   types.List   `tfsdk:"sport"`
	Comment types.String `tfsdk:"comment"`
}

type secmarkAPI struct {
	ID      int64    `json:"id,omitempty"`
	Secmark string   `json:"secmark"`
	Chain   string   `json:"chain"`
	Source  string   `json:"source,omitempty"`
	Dest    string   `json:"dest,omitempty"`
	Proto   string   `json:"proto,omitempty"`
	DPort   []string `json:"dport,omitempty"`
	SPort   []string `json:"sport,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

func NewSecmarkResource() resource.Resource { return &secmarkResource{} }

func (r *secmarkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secmark"
}

func (r *secmarkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An SELinux security-marking rule.",
		Attributes: map[string]schema.Attribute{
			"id":      schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"secmark": schema.StringAttribute{Description: "SELinux context, or save/restore.", Required: true},
			"chain":   schema.StringAttribute{Description: "P/I/F/O/T with optional state.", Required: true},
			"source":  schema.StringAttribute{Optional: true},
			"dest":    schema.StringAttribute{Optional: true},
			"proto":   schema.StringAttribute{Optional: true},
			"dport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"sport":   schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"comment": schema.StringAttribute{Optional: true},
		},
	}
}

func (r *secmarkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *secmarkResource) body(ctx context.Context, plan secmarkModel, diags *diag.Diagnostics) secmarkAPI {
	return secmarkAPI{
		Secmark: plan.Secmark.ValueString(), Chain: plan.Chain.ValueString(), Source: plan.Source.ValueString(),
		Dest: plan.Dest.ValueString(), Proto: plan.Proto.ValueString(),
		DPort: listToStrings(ctx, plan.DPort, diags), SPort: listToStrings(ctx, plan.SPort, diags),
		Comment: plan.Comment.ValueString(),
	}
}

func (r *secmarkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secmarkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out secmarkAPI
	if err := r.client.post(ctx, "/api/v1/secmarks", body, &out); err != nil {
		resp.Diagnostics.AddError("create secmark failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *secmarkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state secmarkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.body(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var out secmarkAPI
	if err := r.client.post(ctx, "/api/v1/secmarks", body, &out); err != nil {
		resp.Diagnostics.AddError("recreate secmark failed", err.Error())
		return
	}
	if id := state.ID.ValueInt64(); id != 0 {
		if err := r.client.del(ctx, "/api/v1/secmarks/"+strconv.FormatInt(id, 10)); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("delete old secmark failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, plan, &resp.Diagnostics))...)
}

func (r *secmarkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secmarkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out secmarkAPI
	if err := r.client.get(ctx, "/api/v1/secmarks/"+strconv.FormatInt(state.ID.ValueInt64(), 10), &out); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read secmark failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.toModel(ctx, out, state, &resp.Diagnostics))...)
}

func (r *secmarkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secmarkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.del(ctx, "/api/v1/secmarks/"+strconv.FormatInt(state.ID.ValueInt64(), 10)); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete secmark failed", err.Error())
	}
}

func (r *secmarkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("invalid import ID", "secmark id must be an integer")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *secmarkResource) toModel(ctx context.Context, api secmarkAPI, prior secmarkModel, diags *diag.Diagnostics) secmarkModel {
	return secmarkModel{
		ID: types.Int64Value(api.ID), Secmark: types.StringValue(api.Secmark), Chain: types.StringValue(api.Chain),
		Source: optionalString(api.Source, prior.Source), Dest: optionalString(api.Dest, prior.Dest),
		Proto: optionalString(api.Proto, prior.Proto), DPort: optionalList(ctx, api.DPort, prior.DPort, diags),
		SPort: optionalList(ctx, api.SPort, prior.SPort, diags), Comment: optionalString(api.Comment, prior.Comment),
	}
}
