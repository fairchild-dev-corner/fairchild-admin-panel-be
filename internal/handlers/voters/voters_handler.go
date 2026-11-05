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
// staff can see who can vote, the current master list and whether the
// election is on; uploading a new list (only while it's on) and turning it
// on or off or resetting everyone to non-voting (both with the admin's
// password) is admin-only.
func (h *VotersHandler) RegisterRoutes(admin *gin.RouterGroup) {
	adminOnly := middlewares.RequireRole(cc.ROLE_ADMIN)
	admin.GET("/voters", h.handleList)
	admin.GET("/voters/master-list", h.handleSummary)
	admin.GET("/voters/stats", h.handleStats)
	admin.POST("/voters/master-list/check", adminOnly, h.handleCheck)
	admin.PUT("/voters/master-list", adminOnly, h.handleSave)
	admin.POST("/voters/master-list/reset", adminOnly, h.handleReset)
	admin.GET("/election-control", h.handleGetControl)
	admin.PUT("/election-control", adminOnly, h.handleSetControl)
}

// handleGetControl: GET /admin/election-control
func (h *VotersHandler) handleGetControl(ctx *gin.Context) {
	c, err := h.votersService.ElectionControl(ctx)
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, c)
}

// handleSetControl: PUT /admin/election-control {enabled, password}
func (h *VotersHandler) handleSetControl(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.SetElectionControlRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	c, err := h.votersService.SetElectionControl(ctx, &req, actor, ctx.Request.UserAgent())
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, c)
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
	// 403, not 401: the panel treats 401 as an expired session and signs out.
	case errors.Is(err, cc.ErrPasswordIncorrect):
		loggers.GetCommonError(ctx, err.Error(), http.StatusForbidden)
	case errors.Is(err, cc.ErrAccountTemporarilyLocked):
		loggers.GetCommonError(ctx, err.Error(), http.StatusTooManyRequests)
	case errors.Is(err, cc.ErrElectionOff):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
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

// handleReset: POST /admin/voters/master-list/reset {password} - everyone
// becomes non-voting.
func (h *VotersHandler) handleReset(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.ResetMasterListRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	s, err := h.votersService.ResetMasterList(ctx, &req, actor, ctx.Request.UserAgent())
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, s)
}

// handleStats: GET /admin/voters/stats - voting and non-voting members per
// branch.
func (h *VotersHandler) handleStats(ctx *gin.Context) {
	stats, err := h.votersService.Stats(ctx)
	if err != nil {
		writeVoterError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, stats)
}
