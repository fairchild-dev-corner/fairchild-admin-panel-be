package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/elections"
	service "fairchild-admin-panel-be/internal/services/elections"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 10
	maxPageSize     = 50
)

type ElectionsHandler struct {
	electionsService *service.ElectionsService
}

func NewHandler(electionsService *service.ElectionsService) *ElectionsHandler {
	return &ElectionsHandler{electionsService: electionsService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Admin and
// staff can see elections, results and who may vote; setting one up,
// publishing, unpublishing, deleting it or uploading its master list is
// admin-only.
func (h *ElectionsHandler) RegisterRoutes(admin *gin.RouterGroup) {
	adminOnly := middlewares.RequireRole(cc.ROLE_ADMIN)
	admin.GET("/elections", h.handleList)
	admin.GET("/elections/candidate-lookup", adminOnly, h.handleLookup)
	admin.GET("/elections/:id", h.handleGet)
	admin.POST("/elections", adminOnly, h.handleCreate)
	admin.PUT("/elections/:id", adminOnly, h.handleUpdate)
	admin.PUT("/elections/:id/publish", adminOnly, h.handlePublish)
	admin.PUT("/elections/:id/unpublish", adminOnly, h.handleUnpublish)
	admin.DELETE("/elections/:id", adminOnly, h.handleDelete)
}

// handleList: GET /admin/elections?status=&page=&pageSize=
func (h *ElectionsHandler) handleList(ctx *gin.Context) {
	q := models.ElectionListQuery{
		Status:   ctx.Query("status"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize),
	}
	if q.Status != "" && !slices.Contains(models.Statuses, q.Status) {
		loggers.GetCommonError(ctx, cc.INVALID_ELECTION_FILTER, http.StatusBadRequest)
		return
	}
	page, err := h.electionsService.List(ctx, q)
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// handleGet: GET /admin/elections/:id - with positions, candidates and votes.
func (h *ElectionsHandler) handleGet(ctx *gin.Context) {
	e, err := h.electionsService.Get(ctx, ctx.Param("id"))
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, e)
}

// handleLookup: GET /admin/elections/candidate-lookup?memberNo=
func (h *ElectionsHandler) handleLookup(ctx *gin.Context) {
	m, err := h.electionsService.LookupCandidate(ctx, ctx.Query("memberNo"))
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

// handleCreate: POST /admin/elections - saved as a draft.
func (h *ElectionsHandler) handleCreate(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.SaveElectionRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	e, err := h.electionsService.Create(ctx, &req, actor)
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, e)
}

// handleUpdate: PUT /admin/elections/:id
func (h *ElectionsHandler) handleUpdate(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.SaveElectionRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	e, err := h.electionsService.Update(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, e)
}

// handlePublish: PUT /admin/elections/:id/publish
func (h *ElectionsHandler) handlePublish(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	e, err := h.electionsService.Publish(ctx, ctx.Param("id"), actor)
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, e)
}

// handleUnpublish: PUT /admin/elections/:id/unpublish
func (h *ElectionsHandler) handleUnpublish(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	e, err := h.electionsService.Unpublish(ctx, ctx.Param("id"), actor)
	if err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, e)
}

// handleDelete: DELETE /admin/elections/:id (drafts only)
func (h *ElectionsHandler) handleDelete(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	if err := h.electionsService.Delete(ctx, ctx.Param("id"), actor); err != nil {
		writeElectionError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, gin.H{"id": ctx.Param("id")})
}

func writeElectionError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrElectionNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrElectionLocked),
		errors.Is(err, cc.ErrElectionNotDraft),
		errors.Is(err, cc.ErrElectionNotPublished):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrElectionInvalid):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}
