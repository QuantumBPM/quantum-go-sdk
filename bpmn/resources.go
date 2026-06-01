package bpmn

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/QuantumBPM/quantum-go-sdk/generated"
	"github.com/QuantumBPM/quantum-go-sdk/variables"
)

// CreateResource stores a new BPMN resource (XML draft) in the project. The
// resource is not deployed until DeployResource is called.
func (c *Client) CreateResource(ctx context.Context, name, xml string) (*ResourceDetail, error) {
	resp, err := c.api.CreateBpmnResourceWithResponse(ctx, c.projectID, generated.CreateBpmnResourceJSONRequestBody{
		Name: name,
		Xml:  xml,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON201 != nil {
		return resp.JSON201, nil
	}
	return nil, errorFromResponses("create-resource", resp.StatusCode())
}

// UpdateResource replaces an existing draft resource with new name and XML.
func (c *Client) UpdateResource(ctx context.Context, resourceID openapi_types.UUID, name, xml string) (*ResourceDetail, error) {
	resp, err := c.api.UpdateBpmnResourceWithResponse(ctx, c.projectID, resourceID, generated.UpdateBpmnResourceJSONRequestBody{
		Name: name,
		Xml:  xml,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("update-resource", resp.StatusCode())
}

// DeleteResource removes a draft resource by its platform UUID.
func (c *Client) DeleteResource(ctx context.Context, resourceID openapi_types.UUID) error {
	resp, err := c.api.DeleteBpmnResourceWithResponse(ctx, c.projectID, resourceID)
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("delete-resource", resp.StatusCode())
}

// GetResource returns the full resource record (including XML) by UUID.
func (c *Client) GetResource(ctx context.Context, resourceID openapi_types.UUID) (*ResourceDetail, error) {
	resp, err := c.api.GetBpmnResourceWithResponse(ctx, c.projectID, resourceID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("get-resource", resp.StatusCode())
}

// DeployResource promotes a stored draft resource to a deployed process
// definition. Subsequent StartInstance calls reference the resulting
// definition.
func (c *Client) DeployResource(ctx context.Context, resourceID openapi_types.UUID) error {
	resp, err := c.api.DeployBpmnResourceWithResponse(ctx, c.projectID, resourceID)
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return nil
	}
	return errorFromResponses("deploy-resource", resp.StatusCode(), resp.JSON400, nil, resp.JSON500)
}

// StartTestInstance starts a non-deployed instance against a draft resource
// for testing.
func (c *Client) StartTestInstance(ctx context.Context, resourceID openapi_types.UUID, processID *string, vars variables.Vars, opts ...StartInstanceOption) (string, error) {
	o := applyStartInstanceOpts(opts...)
	resp, err := c.api.StartBpmnTestInstanceWithResponse(ctx, c.projectID, resourceID, generated.StartBpmnTestInstanceJSONRequestBody{
		ProcessID:  processID,
		Variables:  vars.ToWireMap(),
		BusinessId: o.businessID,
	})
	if err != nil {
		return "", err
	}
	if resp.JSON201 != nil && resp.JSON201.WorkflowID != nil {
		return *resp.JSON201.WorkflowID, nil
	}
	return "", errorFromResponses("start-test-instance", resp.StatusCode())
}

// ValidateXML lints a BPMN XML document, returning per-element issues without
// storing or deploying anything.
func (c *Client) ValidateXML(ctx context.Context, xml string) (*ValidateResponse, error) {
	resp, err := c.api.ValidateBpmnXmlWithResponse(ctx, c.projectID, generated.ValidateBpmnXmlJSONRequestBody{
		Xml: xml,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("validate-xml", resp.StatusCode())
}

// PageOption configures pagination for list endpoints.
type PageOption func(*pageOpts)

type pageOpts struct {
	page     *int
	pageSize *int
}

// WithPage selects the 1-indexed page number.
func WithPage(page int) PageOption { return func(o *pageOpts) { o.page = &page } }

// WithPageSize sets the page size.
func WithPageSize(size int) PageOption { return func(o *pageOpts) { o.pageSize = &size } }

func applyPageOpts(opts ...PageOption) pageOpts {
	o := pageOpts{}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// ListResources returns a page of all resource versions in the project.
func (c *Client) ListResources(ctx context.Context, opts ...PageOption) (*ResourceListPage, error) {
	po := applyPageOpts(opts...)
	resp, err := c.api.ListBpmnResourcesWithResponse(ctx, c.projectID, &generated.ListBpmnResourcesParams{
		Page: po.page, PageSize: po.pageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-resources", resp.StatusCode())
}

// ListLatestResources returns one row per definitions ID, showing the latest
// stored version.
func (c *Client) ListLatestResources(ctx context.Context, opts ...PageOption) (*ResourceSummaryListPage, error) {
	po := applyPageOpts(opts...)
	resp, err := c.api.ListLatestBpmnResourcesWithResponse(ctx, c.projectID, &generated.ListLatestBpmnResourcesParams{
		Page: po.page, PageSize: po.pageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-latest-resources", resp.StatusCode())
}

// ListResourceVersions returns every stored version for a given definitions ID.
func (c *Client) ListResourceVersions(ctx context.Context, definitionsID string, opts ...PageOption) (*ResourceListPage, error) {
	po := applyPageOpts(opts...)
	resp, err := c.api.ListBpmnResourcesByDefinitionsIDWithResponse(ctx, c.projectID, definitionsID, &generated.ListBpmnResourcesByDefinitionsIDParams{
		Page: po.page, PageSize: po.pageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errorFromResponses("list-resource-versions", resp.StatusCode())
}
