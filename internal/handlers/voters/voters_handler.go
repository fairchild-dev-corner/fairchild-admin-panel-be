package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/voters"
	service "fairchild-admin-panel-be/internal/services/voters"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 15
	maxPageSize     = 100
)

type VotersHandler struct {
	votersService *service.VotersService
}

func NewHandler(votersService *service.VotersService) *VotersHandler {
	return &VotersHandler{votersService: votersService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Admin and
// staff can see who can vote and the current master list; uploading a new
// list is admin-only.
func (h *VotersHandler) RegisterRoutes(admin *gin.RouterGroup) {
	adminOnly := middlewares.RequireRole(cc.ROLE_ADMIN)
	admin.GET("/voters", h.handleList)
	admin.GET("/voters/master-list", h.handleSummary)
	admin.POST("/voters/master-list/check", adminOnly, h.handleCheck)
	admin.PUT("/voters/master-list", adminOnly, h.handleSave)
}

// handleList: GET /admin/voters?status=all|eligible|ineligible&q=&page=&pageSize=
func (h *VotersHandler) handleList(ctx *gin.Context) {
	q := models.VoterListQuery{
		Status:   ctx.DefaultQuery("status", models.VoterAll),
		Search:   ctx.Query("q"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize),
	}
	if !slices.Contains(models.VoterStatuses, q.Status) {
		loggers.GetCommonError(ctx, cc.INVALID_VOTER_FILTER, http.StatusBadRequest)
		return
	}
	page, err := h.votersService.List(ctx, q)
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// handleSummary: GET /admin/voters/master-list
func (h *VotersHandler) handleSummary(ctx *gin.Context) {
	s, err := h.votersService.Summary(ctx)
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, s)
}

// handleCheck: POST /admin/voters/master-list/check (multipart, field
// "file") - reads the file and reports which members have a portal account.
// Nothing is saved.
func (h *VotersHandler) handleCheck(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, models.MaxMasterListBytes+(1<<20))
	header, err := ctx.FormFile("file")
	if err != nil {
		loggers.GetCommonError(ctx, cc.MASTER_LIST_NO_FILE, http.StatusBadRequest)
		return
	}
	file, err := header.Open()
	if err != nil {
		loggers.GetCommonError(ctx, cc.MASTER_LIST_NO_FILE, http.StatusBadRequest)
		return
	}
	defer file.Close()

	check, err := h.votersService.CheckMasterList(ctx, header.Filename, file)
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, check)
}

// handleSave: PUT /admin/voters/master-list {fileName, members}
func (h *VotersHandler) handleSave(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.SaveMasterListRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	s, err := h.votersService.SaveMasterList(ctx, &req, actor)
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, s)
}

func writeVoterError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrMasterListInvalid):
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
