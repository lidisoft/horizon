package configdef

import (
	"cmp"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/calendar"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// HTTP serves the configuration endpoints, at workspace scope
// (/w/{ws}/config/…) and project scope (/w/{ws}/projects/{key}/config/…,
// the same shape).
type HTTP struct{ svc *Service }

// NewHTTP returns the handlers.
func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

const targetKey = "horizon.configdef.target"

func targetOf(c *gin.Context) Target { return c.MustGet(targetKey).(Target) }

// RegisterWorkspace mounts the workspace-scope routes on /w/:ws.
func (h *HTTP) RegisterWorkspace(ws *gin.RouterGroup) {
	g := ws.Group("/config", func(c *gin.Context) {
		wc := workspace.Current(c)
		c.Set(targetKey, Target{Principal: wc.Principal, WorkspaceID: wc.Workspace.ID})
	})
	h.register(g)
}

// RegisterProject mounts the project-scope routes on /w/:ws/projects/:key:
// the same as the workspace's, plus switching inherited definitions off and
// copying issue types.
func (h *HTTP) RegisterProject(p *gin.RouterGroup) {
	g := p.Group("/config", func(c *gin.Context) {
		pc := project.Current(c)
		c.Set(targetKey, Target{Principal: pc.Principal, WorkspaceID: pc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: pc.Project.ID, Valid: true}})
	})
	h.register(g)
	for kind, defType := range map[string]string{"categories": TypeCategory, "workflows": TypeWorkflow, "issue-types": TypeIssueType, "fields": TypeCustomField, "labels": TypeLabel} {
		g.POST("/"+kind+"/:id/disable", h.setDisabled(defType, true))
		g.POST("/"+kind+"/:id/enable", h.setDisabled(defType, false))
	}
	g.POST("/issue-types/:id/copy", h.copyIssueType)
	g.GET("/resolved", h.resolved)
	g.POST("/labels/:id/move-to-workspace", h.moveLabel)
}

func (h *HTTP) register(g *gin.RouterGroup) {
	g.GET("/categories", h.categories)
	g.POST("/categories", h.createCategory)
	g.PATCH("/categories/:id", h.updateCategory)
	g.DELETE("/categories/:id", h.deleteCategory)
	g.PUT("/categories/order", h.reorderCategories)
	g.GET("/workflows", h.workflows)
	g.POST("/workflows", h.createWorkflow)
	g.GET("/workflows/:id", h.workflow)
	g.PATCH("/workflows/:id", h.renameWorkflow)
	g.DELETE("/workflows/:id", h.deleteWorkflow)
	g.PUT("/workflows/:id/draft", h.saveDraft)
	g.DELETE("/workflows/:id/draft", h.discardDraft)
	g.POST("/workflows/:id/publish", h.publish)
	g.GET("/issue-types", h.issueTypes)
	g.POST("/issue-types", h.createIssueType)
	g.PATCH("/issue-types/:id", h.updateIssueType)
	g.DELETE("/issue-types/:id", h.deleteIssueType)
	g.PUT("/issue-types/order", h.reorderIssueTypes)
	g.GET("/issue-types/:id/fields", h.typeFields)
	g.PUT("/issue-types/:id/fields", h.setTypeFields)
	g.GET("/fields", h.fields)
	g.POST("/fields", h.createField)
	g.PATCH("/fields/:id", h.updateField)
	g.DELETE("/fields/:id", h.deleteField)
	g.GET("/labels", h.labels)
	g.POST("/labels", h.createLabel)
	g.PATCH("/labels/:id", h.updateLabel)
	g.DELETE("/labels/:id", h.deleteLabel)
	g.POST("/labels/:id/merge", h.mergeLabel)
	g.GET("/settings/estimation", h.estimation)
	g.PUT("/settings/estimation", h.setEstimation)
	g.GET("/settings/iterations", h.iterationSettings)
	g.PUT("/settings/iterations", h.setIterationSettings)
	g.GET("/settings/calendar", h.calendarSettings)
	g.PUT("/settings/calendar", h.setCalendar)
	g.GET("/settings/git", h.gitSettings)
	g.PUT("/settings/git", h.setGitSettings)
}

// GitSettingsResponse is the Git settings in effect and whether the caller
// may change them.
type GitSettingsResponse struct {
	GitSettings
	CanManage bool `json:"can_manage"`
}

// gitSettings returns the Git settings in effect.
//
//	@Summary		Get the Git settings
//	@Description	What a merged pull request does to the issues it names, with the workspace's value and whether the project overrides it.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	GitSettingsResponse
//	@Router			/api/v1/w/{ws}/config/settings/git [get]
func (h *HTTP) gitSettings(c *gin.Context) {
	t := targetOf(c)
	v, err := h.svc.GitSettings(c, h.svc.pool, t)
	reply(c, http.StatusOK, GitSettingsResponse{GitSettings: v, CanManage: h.svc.CanManage(c, t)}, err)
}

// setGitSettings changes the Git settings.
//
//	@Summary		Set the Git settings
//	@Description	merge_status: "@off" (the default), "@completed" (the nearest completed status), or a status name. Null removes the value.
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string				true	"Workspace slug"
//	@Param			body	body		GitSettingsInput	true	"Settings to change"
//	@Success		200		{object}	GitSettings
//	@Router			/api/v1/w/{ws}/config/settings/git [put]
func (h *HTTP) setGitSettings(c *gin.Context) {
	var req GitSettingsInput
	if bind(c, &req) {
		v, err := h.svc.SetGitSettings(c, targetOf(c), req)
		reply(c, http.StatusOK, v, err)
	}
}

// CategoryList is the body of GET …/config/categories.
type CategoryList = ListResponse[Category]

// WorkflowList is the body of GET …/config/workflows.
type WorkflowList = ListResponse[Workflow]

// IssueTypeList is the body of GET …/config/issue-types.
type IssueTypeList = ListResponse[IssueType]

// categories lists status categories.
//
//	@Summary		List status categories
//	@Description	Same at project scope: /w/{ws}/projects/{key}/config/categories, with inherited ones flagged.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	CategoryList
//	@Router			/api/v1/w/{ws}/config/categories [get]
func (h *HTTP) categories(c *gin.Context) {
	t := targetOf(c)
	items, err := h.svc.Categories(c, t)
	list(c, items, err, h.svc.CanManage(c, t))
}

// createCategory adds a status category.
//
//	@Summary	Create a status category
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string			true	"Workspace slug"
//	@Param		body	body		CategoryRequest	true	"Name, colour, outcome"
//	@Success	201		{object}	Category
//	@Router		/api/v1/w/{ws}/config/categories [post]
func (h *HTTP) createCategory(c *gin.Context) {
	var req CategoryRequest
	if bind(c, &req) {
		v, err := h.svc.CreateCategory(c, targetOf(c), CategoryInput(req))
		reply(c, http.StatusCreated, v, err)
	}
}

// updateCategory changes a status category.
//
//	@Summary		Update a status category
//	@Description	A category holding a workflow's initial status can't become final (409 initial_status_in_category).
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string			true	"Workspace slug"
//	@Param			id			path		string			true	"Category id"
//	@Param			If-Match	header		string			false	"Category version"
//	@Param			body		body		CategoryRequest	true	"Fields to change"
//	@Success		200			{object}	Category
//	@Router			/api/v1/w/{ws}/config/categories/{id} [patch]
func (h *HTTP) updateCategory(c *gin.Context) {
	var req CategoryRequest
	id, ok := idParam(c, "category_not_found")
	if ok && bind(c, &req) {
		if version, ok := ifMatch(c); ok {
			v, err := h.svc.UpdateCategory(c, targetOf(c), id, version, CategoryInput(req))
			reply(c, http.StatusOK, v, err)
		}
	}
}

// deleteCategory removes a status category.
//
//	@Summary		Delete a status category
//	@Description	A category in use needs ?replacement= with the same kind of outcome (open or final); its statuses move there in every workflow (409 replacement_required otherwise).
//	@Tags			configuration
//	@Param			ws			path	string	true	"Workspace slug"
//	@Param			id			path	string	true	"Category id"
//	@Param			replacement	query	string	false	"Category taking over its statuses"
//	@Success		204
//	@Router			/api/v1/w/{ws}/config/categories/{id} [delete]
func (h *HTTP) deleteCategory(c *gin.Context) {
	id, ok := idParam(c, "category_not_found")
	if !ok {
		return
	}
	var replacement uuid.NullUUID
	if r := c.Query("replacement"); r != "" {
		rid, err := uuid.Parse(r)
		if err != nil {
			server.WriteError(c, apperr.Invalid(apperr.Field{Field: "replacement", Code: "unknown"}))
			return
		}
		replacement = uuid.NullUUID{UUID: rid, Valid: true}
	}
	noContent(c, h.svc.DeleteCategory(c, targetOf(c), id, replacement))
}

// reorderCategories sets the board order.
//
//	@Summary	Reorder status categories
//	@Tags		configuration
//	@Accept		json
//	@Param		ws		path	string			true	"Workspace slug"
//	@Param		body	body	OrderRequest	true	"Category ids in board order"
//	@Success	204
//	@Router		/api/v1/w/{ws}/config/categories/order [put]
func (h *HTTP) reorderCategories(c *gin.Context) {
	var req OrderRequest
	if bind(c, &req) {
		noContent(c, h.svc.ReorderCategories(c, targetOf(c), req.IDs))
	}
}

// workflows lists workflows.
//
//	@Summary	List workflows
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	WorkflowList
//	@Router		/api/v1/w/{ws}/config/workflows [get]
func (h *HTTP) workflows(c *gin.Context) {
	t := targetOf(c)
	items, err := h.svc.Workflows(c, t)
	list(c, items, err, h.svc.CanManage(c, t))
}

// createWorkflow copies a workflow under a new name.
//
//	@Summary	Create a workflow (a copy)
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string					true	"Workspace slug"
//	@Param		body	body		CreateWorkflowRequest	true	"Name and the workflow to copy"
//	@Success	201		{object}	Workflow
//	@Router		/api/v1/w/{ws}/config/workflows [post]
func (h *HTTP) createWorkflow(c *gin.Context) {
	var req CreateWorkflowRequest
	if bind(c, &req) {
		v, err := h.svc.CreateWorkflow(c, targetOf(c), req.Name, req.CopyFrom)
		reply(c, http.StatusCreated, v, err)
	}
}

// workflow returns a workflow with its draft.
//
//	@Summary	Get a workflow
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Workflow id"
//	@Success	200	{object}	Workflow
//	@Router		/api/v1/w/{ws}/config/workflows/{id} [get]
func (h *HTTP) workflow(c *gin.Context) {
	if id, ok := idParam(c, "workflow_not_found"); ok {
		v, err := h.svc.Workflow(c, targetOf(c), id)
		reply(c, http.StatusOK, v, err)
	}
}

// renameWorkflow renames a workflow.
//
//	@Summary	Rename a workflow
//	@Tags		configuration
//	@Accept		json
//	@Param		ws		path	string			true	"Workspace slug"
//	@Param		id		path	string			true	"Workflow id"
//	@Param		body	body	RenameRequest	true	"New name"
//	@Success	204
//	@Router		/api/v1/w/{ws}/config/workflows/{id} [patch]
func (h *HTTP) renameWorkflow(c *gin.Context) {
	var req RenameRequest
	id, ok := idParam(c, "workflow_not_found")
	if ok && bind(c, &req) {
		noContent(c, h.svc.RenameWorkflow(c, targetOf(c), id, req.Name))
	}
}

// deleteWorkflow removes an unused workflow.
//
//	@Summary	Delete a workflow
//	@Tags		configuration
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Workflow id"
//	@Success	204
//	@Failure	409	{object}	server.Problem	"workflow_in_use"
//	@Router		/api/v1/w/{ws}/config/workflows/{id} [delete]
func (h *HTTP) deleteWorkflow(c *gin.Context) {
	if id, ok := idParam(c, "workflow_not_found"); ok {
		noContent(c, h.svc.DeleteWorkflow(c, targetOf(c), id))
	}
}

// saveDraft stores the shared draft.
//
//	@Summary		Save a workflow draft
//	@Description	The whole draft document; version is the draft_version it was based on (409 version_conflict if someone saved since).
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			id		path		string			true	"Workflow id"
//	@Param			body	body		DraftRequest	true	"Draft and base version"
//	@Success		200		{object}	Workflow
//	@Router			/api/v1/w/{ws}/config/workflows/{id}/draft [put]
func (h *HTTP) saveDraft(c *gin.Context) {
	var req DraftRequest
	id, ok := idParam(c, "workflow_not_found")
	if ok && bind(c, &req) {
		v, err := h.svc.SaveDraft(c, targetOf(c), id, req.Version, req.Draft)
		reply(c, http.StatusOK, v, err)
	}
}

// discardDraft drops the draft.
//
//	@Summary	Discard a workflow draft
//	@Tags		configuration
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Workflow id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/config/workflows/{id}/draft [delete]
func (h *HTTP) discardDraft(c *gin.Context) {
	if id, ok := idParam(c, "workflow_not_found"); ok {
		noContent(c, h.svc.DiscardDraft(c, targetOf(c), id))
	}
}

// publish applies the draft.
//
//	@Summary		Publish a workflow draft
//	@Description	409 unmapped_statuses lists each removed status holding issues (field status_mapping.<id>, code required:<count>).
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			id		path		string			true	"Workflow id"
//	@Param			body	body		PublishRequest	true	"Draft version and status mapping"
//	@Success		200		{object}	PublishResult
//	@Failure		409		{object}	server.Problem
//	@Router			/api/v1/w/{ws}/config/workflows/{id}/publish [post]
func (h *HTTP) publish(c *gin.Context) {
	var req PublishRequest
	id, ok := idParam(c, "workflow_not_found")
	if ok && bind(c, &req) {
		v, err := h.svc.Publish(c, targetOf(c), id, req.Version, req.StatusMapping)
		reply(c, http.StatusOK, v, err)
	}
}

// issueTypes lists issue types.
//
//	@Summary	List issue types
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	IssueTypeList
//	@Router		/api/v1/w/{ws}/config/issue-types [get]
func (h *HTTP) issueTypes(c *gin.Context) {
	t := targetOf(c)
	items, err := h.svc.IssueTypes(c, t)
	list(c, items, err, h.svc.CanManage(c, t))
}

// createIssueType adds an issue type.
//
//	@Summary	Create an issue type
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string				true	"Workspace slug"
//	@Param		body	body		IssueTypeRequest	true	"Name, icon, colour, workflow, parent rule"
//	@Success	201		{object}	IssueType
//	@Router		/api/v1/w/{ws}/config/issue-types [post]
func (h *HTTP) createIssueType(c *gin.Context) {
	var req IssueTypeRequest
	if bind(c, &req) {
		v, err := h.svc.CreateIssueType(c, targetOf(c), req.input())
		reply(c, http.StatusCreated, v, err)
	}
}

// updateIssueType changes an issue type.
//
//	@Summary		Update an issue type
//	@Description	Moving it to another workflow needs status_mapping for statuses holding its issues (409 unmapped_statuses).
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws			path		string				true	"Workspace slug"
//	@Param			id			path		string				true	"Issue type id"
//	@Param			If-Match	header		string				false	"Issue type version"
//	@Param			body		body		IssueTypeRequest	true	"Fields to change"
//	@Success		200			{object}	IssueType
//	@Router			/api/v1/w/{ws}/config/issue-types/{id} [patch]
func (h *HTTP) updateIssueType(c *gin.Context) {
	var req IssueTypeRequest
	id, ok := idParam(c, "issue_type_not_found")
	if ok && bind(c, &req) {
		if version, ok := ifMatch(c); ok {
			v, err := h.svc.UpdateIssueType(c, targetOf(c), id, version, req.input(), req.StatusMapping)
			reply(c, http.StatusOK, v, err)
		}
	}
}

// deleteIssueType removes an unused issue type.
//
//	@Summary	Delete an issue type
//	@Tags		configuration
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Issue type id"
//	@Success	204
//	@Failure	409	{object}	server.Problem	"issue_type_in_use"
//	@Router		/api/v1/w/{ws}/config/issue-types/{id} [delete]
func (h *HTTP) deleteIssueType(c *gin.Context) {
	if id, ok := idParam(c, "issue_type_not_found"); ok {
		noContent(c, h.svc.DeleteIssueType(c, targetOf(c), id))
	}
}

// reorderIssueTypes sets the order of issue types.
//
//	@Summary	Reorder issue types
//	@Tags		configuration
//	@Accept		json
//	@Param		ws		path	string			true	"Workspace slug"
//	@Param		body	body	OrderRequest	true	"Issue type ids in order"
//	@Success	204
//	@Router		/api/v1/w/{ws}/config/issue-types/order [put]
func (h *HTTP) reorderIssueTypes(c *gin.Context) {
	var req OrderRequest
	if bind(c, &req) {
		noContent(c, h.svc.ReorderIssueTypes(c, targetOf(c), req.IDs))
	}
}

// copyIssueType copies an inherited issue type into the project.
//
//	@Summary		Copy an inherited issue type into a project
//	@Description	The copy is editable; the original is switched off in the project.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Param			key	path		string	true	"Project key"
//	@Param			id	path		string	true	"Issue type id"
//	@Success		201	{object}	IssueType
//	@Router			/api/v1/w/{ws}/projects/{key}/config/issue-types/{id}/copy [post]
func (h *HTTP) copyIssueType(c *gin.Context) {
	if id, ok := idParam(c, "issue_type_not_found"); ok {
		v, err := h.svc.CopyIssueType(c, targetOf(c), id)
		reply(c, http.StatusCreated, v, err)
	}
}

// setDisabled switches an inherited definition off or on in a project:
// POST /w/{ws}/projects/{key}/config/{categories|workflows|issue-types}/{id}/disable|enable.
func (h *HTTP) setDisabled(defType string, off bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := idParam(c, defType+"_not_found"); ok {
			noContent(c, h.svc.SetDisabled(c, targetOf(c), defType, id, off))
		}
	}
}

// FieldList is the body of GET …/config/fields.
type FieldList = ListResponse[Field]

// LabelList is the body of GET …/config/labels.
type LabelList = ListResponse[Label]

// FieldRequest creates or changes a custom field.
type FieldRequest struct {
	Name        *string   `json:"name,omitempty" example:"Severity"`
	Type        *string   `json:"type,omitempty" enums:"text,number,select,multi_select,date,user,url,checkbox"`
	Description *string   `json:"description,omitempty"`
	Options     *[]Option `json:"options,omitempty"`
	Unit        *string   `json:"unit,omitempty"`
	Precision   *int      `json:"precision,omitempty"`
}

// TypeFieldsRequest lists an issue type's fields in display order.
type TypeFieldsRequest struct {
	Fields []TypeField `json:"fields"`
}

// LabelRequest creates or changes a label.
type LabelRequest struct {
	Name        *string `json:"name,omitempty" example:"backend"`
	Colour      *string `json:"colour,omitempty"`
	Description *string `json:"description,omitempty"`
}

// MergeRequest names the label taking over.
type MergeRequest struct {
	Into uuid.UUID `json:"into"`
}

// EstimationRequest sets the scale; at project scope, null scale removes
// the override.
type EstimationRequest struct {
	Scale *Scale `json:"scale"`
}

// fields lists custom fields.
//
//	@Summary	List custom fields
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	FieldList
//	@Router		/api/v1/w/{ws}/config/fields [get]
func (h *HTTP) fields(c *gin.Context) {
	t := targetOf(c)
	items, err := h.svc.Fields(c, t)
	list(c, items, err, h.svc.CanManage(c, t))
}

// createField adds a custom field.
//
//	@Summary	Create a custom field
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string			true	"Workspace slug"
//	@Param		body	body		FieldRequest	true	"Name, type and type options"
//	@Success	201		{object}	Field
//	@Router		/api/v1/w/{ws}/config/fields [post]
func (h *HTTP) createField(c *gin.Context) {
	var req FieldRequest
	if bind(c, &req) {
		v, err := h.svc.CreateField(c, targetOf(c), FieldInput(req))
		reply(c, http.StatusCreated, v, err)
	}
}

// updateField changes a custom field; its type is fixed.
//
//	@Summary	Update a custom field
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string			true	"Workspace slug"
//	@Param		id			path		string			true	"Field id"
//	@Param		If-Match	header		string			false	"Field version"
//	@Param		body		body		FieldRequest	true	"Fields to change"
//	@Success	200			{object}	Field
//	@Router		/api/v1/w/{ws}/config/fields/{id} [patch]
func (h *HTTP) updateField(c *gin.Context) {
	var req FieldRequest
	id, ok := idParam(c, "field_not_found")
	if ok && bind(c, &req) {
		if version, ok := ifMatch(c); ok {
			v, err := h.svc.UpdateField(c, targetOf(c), id, version, FieldInput(req))
			reply(c, http.StatusOK, v, err)
		}
	}
}

// deleteField removes a custom field.
//
//	@Summary		Delete a custom field
//	@Description	Detaches it from issue types and drops it from workflow rules.
//	@Tags			configuration
//	@Param			ws	path	string	true	"Workspace slug"
//	@Param			id	path	string	true	"Field id"
//	@Success		204
//	@Router			/api/v1/w/{ws}/config/fields/{id} [delete]
func (h *HTTP) deleteField(c *gin.Context) {
	if id, ok := idParam(c, "field_not_found"); ok {
		noContent(c, h.svc.DeleteField(c, targetOf(c), id))
	}
}

// typeFields lists an issue type's fields.
//
//	@Summary	List the fields of an issue type
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Param		id	path		string	true	"Issue type id"
//	@Success	200	{object}	TypeFieldsRequest
//	@Router		/api/v1/w/{ws}/config/issue-types/{id}/fields [get]
func (h *HTTP) typeFields(c *gin.Context) {
	if id, ok := idParam(c, "issue_type_not_found"); ok {
		v, err := h.svc.TypeFields(c, targetOf(c), id)
		reply(c, http.StatusOK, TypeFieldsRequest{Fields: v}, err)
	}
}

// setTypeFields replaces an issue type's fields.
//
//	@Summary		Set the fields of an issue type
//	@Description	Fields the type's scope sees, in display order; required ones are checked when an issue is created.
//	@Tags			configuration
//	@Accept			json
//	@Param			ws		path	string				true	"Workspace slug"
//	@Param			id		path	string				true	"Issue type id"
//	@Param			body	body	TypeFieldsRequest	true	"Fields in order"
//	@Success		204
//	@Router			/api/v1/w/{ws}/config/issue-types/{id}/fields [put]
func (h *HTTP) setTypeFields(c *gin.Context) {
	var req TypeFieldsRequest
	id, ok := idParam(c, "issue_type_not_found")
	if ok && bind(c, &req) {
		noContent(c, h.svc.SetTypeFields(c, targetOf(c), id, req.Fields))
	}
}

// labels lists labels with their issue counts.
//
//	@Summary	List labels
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	LabelList
//	@Router		/api/v1/w/{ws}/config/labels [get]
func (h *HTTP) labels(c *gin.Context) {
	t := targetOf(c)
	items, err := h.svc.Labels(c, t)
	list(c, items, err, h.svc.CanManage(c, t))
}

// createLabel adds a label.
//
//	@Summary	Create a label
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws		path		string			true	"Workspace slug"
//	@Param		body	body		LabelRequest	true	"Name, colour, description"
//	@Success	201		{object}	Label
//	@Router		/api/v1/w/{ws}/config/labels [post]
func (h *HTTP) createLabel(c *gin.Context) {
	var req LabelRequest
	if bind(c, &req) {
		v, err := h.svc.CreateLabel(c, targetOf(c), LabelInput(req))
		reply(c, http.StatusCreated, v, err)
	}
}

// updateLabel changes a label.
//
//	@Summary	Update a label
//	@Tags		configuration
//	@Accept		json
//	@Produce	json
//	@Param		ws			path		string			true	"Workspace slug"
//	@Param		id			path		string			true	"Label id"
//	@Param		If-Match	header		string			false	"Label version"
//	@Param		body		body		LabelRequest	true	"Fields to change"
//	@Success	200			{object}	Label
//	@Router		/api/v1/w/{ws}/config/labels/{id} [patch]
func (h *HTTP) updateLabel(c *gin.Context) {
	var req LabelRequest
	id, ok := idParam(c, "label_not_found")
	if ok && bind(c, &req) {
		if version, ok := ifMatch(c); ok {
			v, err := h.svc.UpdateLabel(c, targetOf(c), id, version, LabelInput(req))
			reply(c, http.StatusOK, v, err)
		}
	}
}

// deleteLabel removes a label.
//
//	@Summary	Delete a label
//	@Tags		configuration
//	@Param		ws	path	string	true	"Workspace slug"
//	@Param		id	path	string	true	"Label id"
//	@Success	204
//	@Router		/api/v1/w/{ws}/config/labels/{id} [delete]
func (h *HTTP) deleteLabel(c *gin.Context) {
	if id, ok := idParam(c, "label_not_found"); ok {
		noContent(c, h.svc.DeleteLabel(c, targetOf(c), id))
	}
}

// mergeLabel relabels issues with another label, then deletes this one.
//
//	@Summary		Merge a label into another
//	@Description	Every issue carrying it gets the other label instead, recorded in its history.
//	@Tags			configuration
//	@Accept			json
//	@Param			ws		path	string			true	"Workspace slug"
//	@Param			id		path	string			true	"Label id"
//	@Param			body	body	MergeRequest	true	"Label taking over"
//	@Success		204
//	@Router			/api/v1/w/{ws}/config/labels/{id}/merge [post]
func (h *HTTP) mergeLabel(c *gin.Context) {
	var req MergeRequest
	id, ok := idParam(c, "label_not_found")
	if ok && bind(c, &req) {
		noContent(c, h.svc.MergeLabel(c, targetOf(c), id, req.Into))
	}
}

// moveLabel promotes a project label to the workspace.
//
//	@Summary		Move a project label to the workspace
//	@Description	Needs workspace.config.manage.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Param			key	path		string	true	"Project key"
//	@Param			id	path		string	true	"Label id"
//	@Success		200	{object}	Label
//	@Router			/api/v1/w/{ws}/projects/{key}/config/labels/{id}/move-to-workspace [post]
func (h *HTTP) moveLabel(c *gin.Context) {
	if id, ok := idParam(c, "label_not_found"); ok {
		v, err := h.svc.MoveLabelToWorkspace(c, targetOf(c), id)
		reply(c, http.StatusOK, v, err)
	}
}

// EstimationResponse is the effective scale and whether the caller may
// change it.
type EstimationResponse struct {
	Estimation
	CanManage bool `json:"can_manage"`
}

// estimation returns the effective estimation scale.
//
//	@Summary	Get the estimation scale
//	@Tags		configuration
//	@Produce	json
//	@Param		ws	path		string	true	"Workspace slug"
//	@Success	200	{object}	EstimationResponse
//	@Router		/api/v1/w/{ws}/config/settings/estimation [get]
func (h *HTTP) estimation(c *gin.Context) {
	t := targetOf(c)
	e, err := h.svc.Estimation(c, t)
	reply(c, http.StatusOK, EstimationResponse{Estimation: e, CanManage: h.svc.CanManage(c, t)}, err)
}

// setEstimation sets the estimation scale.
//
//	@Summary		Set the estimation scale
//	@Description	At project scope, {"scale": null} returns the project to the workspace's scale.
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string				true	"Workspace slug"
//	@Param			body	body		EstimationRequest	true	"Scale"
//	@Success		200		{object}	Estimation
//	@Router			/api/v1/w/{ws}/config/settings/estimation [put]
func (h *HTTP) setEstimation(c *gin.Context) {
	var req EstimationRequest
	if bind(c, &req) {
		v, err := h.svc.SetEstimation(c, targetOf(c), req.Scale)
		reply(c, http.StatusOK, v, err)
	}
}

// IterationSettingsResponse is the effective iteration settings and
// whether the caller may change them.
type IterationSettingsResponse struct {
	IterationSettings
	CanManage bool `json:"can_manage"`
}

// IterationSettingsRequest changes iteration settings. An absent key is
// left alone; null removes the value, so a project follows the workspace
// again and the workspace returns to the built-in default.
type IterationSettingsRequest struct {
	CompletionPolicy     *string `json:"completion_policy,omitempty" enums:"archive_done_rollover_rest,archive_all,none"`
	MaxActiveIterations  *int    `json:"max_active_iterations,omitempty" minimum:"1" maximum:"10"`
	IterationLengthWeeks *int    `json:"iteration_length_weeks,omitempty" minimum:"1" maximum:"8"`
}

// iterationSettings returns the iteration settings in effect.
//
//	@Summary		Get the iteration settings
//	@Description	Completion policy, maximum active iterations and default iteration length, each with the workspace's value and whether the project overrides it.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	IterationSettingsResponse
//	@Router			/api/v1/w/{ws}/config/settings/iterations [get]
func (h *HTTP) iterationSettings(c *gin.Context) {
	t := targetOf(c)
	v, err := h.svc.IterationSettings(c, h.svc.pool, t)
	reply(c, http.StatusOK, IterationSettingsResponse{IterationSettings: v, CanManage: h.svc.CanManage(c, t)}, err)
}

// setIterationSettings changes iteration settings.
//
//	@Summary		Set iteration settings
//	@Description	Keys left out are unchanged; null removes a value (at project scope, the project follows the workspace again).
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string						true	"Workspace slug"
//	@Param			body	body		IterationSettingsRequest	true	"Settings to change"
//	@Success		200		{object}	IterationSettings
//	@Router			/api/v1/w/{ws}/config/settings/iterations [put]
func (h *HTTP) setIterationSettings(c *gin.Context) {
	var req IterationSettingsInput
	if bind(c, &req) {
		v, err := h.svc.SetIterationSettings(c, targetOf(c), req)
		reply(c, http.StatusOK, v, err)
	}
}

// CalendarResponse is the working calendar in effect and whether the
// caller may change it.
type CalendarResponse struct {
	CalendarSettings
	CanManage bool `json:"can_manage"`
}

// CalendarRequest changes the working calendar. An absent key is left
// alone; null removes the value, so a project follows the workspace again
// and the workspace returns to the built-in default.
type CalendarRequest struct {
	// TimeZone overrides the workspace's at project scope (the workspace's
	// is changed with PATCH /w/{ws}).
	TimeZone *string `json:"time_zone,omitempty" example:"Europe/Paris"`
	// WorkingDays are ISO weekdays, Monday 1 to Sunday 7.
	WorkingDays  []int              `json:"working_days,omitempty" example:"1,2,3,4,5"`
	Holidays     []calendar.Holiday `json:"holidays,omitempty"`
	HolidaysMode *string            `json:"holidays_mode,omitempty" enums:"add,replace"`
}

// calendarSettings returns the working calendar in effect.
//
//	@Summary		Get the working calendar
//	@Description	Time zone, working weekdays and non-working dates, each with the workspace's value and whether (or how) the project overrides it.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Success		200	{object}	CalendarResponse
//	@Router			/api/v1/w/{ws}/config/settings/calendar [get]
func (h *HTTP) calendarSettings(c *gin.Context) {
	t := targetOf(c)
	v, err := h.svc.CalendarSettings(c, h.svc.pool, t)
	reply(c, http.StatusOK, CalendarResponse{CalendarSettings: v, CanManage: h.svc.CanManage(c, t)}, err)
}

// setCalendar changes the working calendar.
//
//	@Summary		Set the working calendar
//	@Description	Keys left out are unchanged; null removes a value (at project scope, the project follows the workspace again). A project's holidays add to the workspace's or replace them (holidays_mode).
//	@Tags			configuration
//	@Accept			json
//	@Produce		json
//	@Param			ws		path		string			true	"Workspace slug"
//	@Param			body	body		CalendarRequest	true	"Settings to change"
//	@Success		200		{object}	CalendarSettings
//	@Router			/api/v1/w/{ws}/config/settings/calendar [put]
func (h *HTTP) setCalendar(c *gin.Context) {
	var req CalendarInput
	if bind(c, &req) {
		v, err := h.svc.SetCalendar(c, targetOf(c), req)
		reply(c, http.StatusOK, v, err)
	}
}

// --- Requests and helpers -------------------------------------------------------

// CategoryRequest creates or changes a status category.
type CategoryRequest struct {
	Name    *string `json:"name,omitempty" example:"In review"`
	Colour  *string `json:"colour,omitempty"`
	Outcome *string `json:"outcome,omitempty" enums:"unstarted,started,completed,abandoned"`
}

// OrderRequest lists ids in their new order.
type OrderRequest struct {
	IDs []uuid.UUID `json:"ids"`
}

// CreateWorkflowRequest copies a workflow under a new name.
type CreateWorkflowRequest struct {
	Name     string    `json:"name" example:"Engineering"`
	CopyFrom uuid.UUID `json:"copy_from"`
}

// RenameRequest renames a definition.
type RenameRequest struct {
	Name string `json:"name"`
}

// DraftRequest saves the shared draft; version is the draft_version it
// was based on.
type DraftRequest struct {
	Version int32 `json:"version"`
	Draft   Draft `json:"draft"`
}

// PublishRequest applies the draft; status_mapping sends the issues of each
// removed status to a status of the draft.
type PublishRequest struct {
	Version       int32                   `json:"version"`
	StatusMapping map[uuid.UUID]uuid.UUID `json:"status_mapping,omitempty"`
}

// IssueTypeRequest creates or changes an issue type; status_mapping is
// needed when moving a type with issues to another workflow.
type IssueTypeRequest struct {
	Name          *string                 `json:"name,omitempty" example:"Bug"`
	Icon          *string                 `json:"icon,omitempty"`
	Colour        *string                 `json:"colour,omitempty"`
	WorkflowID    *uuid.UUID              `json:"workflow_id,omitempty"`
	ParentRule    *string                 `json:"parent_rule,omitempty" enums:"any,none,required"`
	ParentTypeIDs *[]uuid.UUID            `json:"parent_type_ids,omitempty"`
	StatusMapping map[uuid.UUID]uuid.UUID `json:"status_mapping,omitempty"`
}

func (r IssueTypeRequest) input() IssueTypeInput {
	return IssueTypeInput{Name: r.Name, Icon: r.Icon, Colour: r.Colour, WorkflowID: r.WorkflowID, ParentRule: r.ParentRule, ParentTypeIDs: r.ParentTypeIDs}
}

// ListResponse is the body of the list endpoints.
type ListResponse[T any] struct {
	Items []T `json:"items"`
	// CanManage tells whether the caller may edit this scope's definitions.
	CanManage bool `json:"can_manage"`
}

func list[T any](c *gin.Context, items []T, err error, canManage bool) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	if items == nil {
		items = []T{}
	}
	c.JSON(http.StatusOK, ListResponse[T]{Items: items, CanManage: canManage})
}

func reply(c *gin.Context, status int, v any, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.JSON(status, v)
}

func noContent(c *gin.Context, err error) {
	if err != nil {
		server.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bind(c *gin.Context, v any) bool {
	if err := server.BindJSON(c, v); err != nil {
		server.WriteError(c, err)
		return false
	}
	return true
}

func idParam(c *gin.Context, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		server.WriteError(c, apperr.NotFound(notFound))
		return uuid.Nil, false
	}
	return id, true
}

func ifMatch(c *gin.Context) (int32, bool) {
	v, err := server.IfMatch(c)
	if err != nil {
		server.WriteError(c, err)
		return 0, false
	}
	return v, true
}

// ResolvedConfig is the configuration a project's issues use: what the
// project inherits and what it overrides, without what it switched off.
type ResolvedConfig struct {
	Categories []Category  `json:"categories"`
	Workflows  []Workflow  `json:"workflows"`
	IssueTypes []IssueType `json:"issue_types"`
	Fields     []Field     `json:"fields"`
	// TypeFields lists each issue type's fields, in order, by type id.
	TypeFields map[uuid.UUID][]TypeField `json:"type_fields" swaggertype:"object"`
	// Labels include those switched off in the project (disabled): issues
	// keep them, greyed, but they aren't offered.
	Labels     []Label    `json:"labels"`
	Estimation Estimation `json:"estimation"`
	// Calendar is the project's working calendar.
	Calendar CalendarSettings `json:"calendar"`
}

// resolved returns the project's resolved configuration.
//
//	@Summary		Resolved configuration of a project
//	@Description	For issue screens: categories, published workflows, issue types, fields per type, labels and the estimation scale in effect. Readable by whoever sees the project.
//	@Tags			configuration
//	@Produce		json
//	@Param			ws	path		string	true	"Workspace slug"
//	@Param			key	path		string	true	"Project key"
//	@Success		200	{object}	ResolvedConfig
//	@Router			/api/v1/w/{ws}/projects/{key}/config/resolved [get]
func (h *HTTP) resolved(c *gin.Context) {
	t := targetOf(c)
	r, err := h.svc.Resolve(c, h.svc.pool, t.WorkspaceID, t.ProjectID)
	if err != nil {
		server.WriteError(c, err)
		return
	}
	out := ResolvedConfig{IssueTypes: r.IssueTypes, TypeFields: r.TypeFields, Estimation: r.Estimation}
	if out.Calendar, err = h.svc.CalendarSettings(c, h.svc.pool, t); err != nil {
		server.WriteError(c, err)
		return
	}
	for _, x := range r.Categories {
		out.Categories = append(out.Categories, x)
	}
	slices.SortFunc(out.Categories, func(a, b Category) int { return cmp.Compare(a.Position, b.Position) })
	for _, w := range r.Workflows {
		w.Draft = nil
		out.Workflows = append(out.Workflows, w)
	}
	slices.SortFunc(out.Workflows, func(a, b Workflow) int { return strings.Compare(a.Name, b.Name) })
	for _, f := range r.Fields {
		out.Fields = append(out.Fields, f)
	}
	slices.SortFunc(out.Fields, func(a, b Field) int { return cmp.Compare(a.Position, b.Position) })
	if out.Labels, err = h.svc.Labels(c, t); err != nil {
		server.WriteError(c, err)
		return
	}
	if out.TypeFields == nil {
		out.TypeFields = map[uuid.UUID][]TypeField{}
	}
	if out.Categories == nil {
		out.Categories = []Category{}
	}
	if out.Workflows == nil {
		out.Workflows = []Workflow{}
	}
	if out.Fields == nil {
		out.Fields = []Field{}
	}
	if out.Labels == nil {
		out.Labels = []Label{}
	}
	if out.IssueTypes == nil {
		out.IssueTypes = []IssueType{}
	}
	c.JSON(http.StatusOK, out)
}
