package handlers

import (
	"errors"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"net/http"
	"strconv"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/blasts"
	service "fairchild-admin-panel-be/internal/services/blasts"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 10
	maxPageSize     = 50
)

type BlastsHandler struct {
	blastsService *service.BlastsService
}

func NewHandler(blastsService *service.BlastsService) *BlastsHandler {
	return &BlastsHandler{blastsService: blastsService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Sending,
// testing or cancelling a blast - which messages members - is admins only
// unless allowed for staff on the Capabilities page. Previewing the audience
// goes with it, since only someone who can send acts on it.
func (h *BlastsHandler) RegisterRoutes(admin *gin.RouterGroup) {
	need := middlewares.RequireCapability
	admin.GET("/blasts", need(caps.BlastsView), h.handleList)
	admin.GET("/blasts/:id", need(caps.BlastsView), h.handleGet)
	admin.POST("/blasts/preview", need(caps.BlastsSend), h.handlePreview)
	admin.POST("/blasts/test", need(caps.BlastsSend), h.handleTest)
	admin.POST("/blasts", need(caps.BlastsSend), h.handleCreate)
	admin.PUT("/blasts/:id/cancel", need(caps.BlastsSend), h.handleCancel)
}

// handleList: GET /admin/blasts?page=&pageSize=
func (h *BlastsHandler) handleList(ctx *gin.Context) {
	page := max(1, atoiOr(ctx.Query("page"), 1))
	pageSize := min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize)

	result, err := h.blastsService.List(ctx, page, pageSize)
	if err != nil {
		writeBlastError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, result)
}

// handleGet: GET /admin/blasts/:id
func (h *BlastsHandler) handleGet(ctx *gin.Context) {
	b, err := h.blastsService.Get(ctx, ctx.Param("id"))
	if err != nil {
		writeBlastError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, b)
}

// handlePreview: POST /admin/blasts/preview {channel, statuses, branches}
func (h *BlastsHandler) handlePreview(ctx *gin.Context) {
	var req models.AudienceRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	p, err := h.blastsService.Preview(ctx, req)
	if err != nil {
		writeBlastError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, p)
}

// handleTest: POST /admin/blasts/test {channel, subject, emailBody, smsBody, email, mobile}
func (h *BlastsHandler) handleTest(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.TestBlastRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	result, err := h.blastsService.SendTest(ctx, &req, actor)
	if err != nil {
		writeBlastError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, result)
}

// handleCreate: POST /admin/blasts {channel, statuses, branches, subject, emailBody, smsBody}
func (h *BlastsHandler) handleCreate(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	var req models.CreateBlastRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}
	b, err := h.blastsService.Create(ctx, &req, actor)
	if err != nil {
		writeBlastError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, b)
}

// handleCancel: PUT /admin/blasts/:id/cancel
func (h *BlastsHandler) handleCancel(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	b, err := h.blastsService.Cancel(ctx, ctx.Param("id"), actor)
	if err != nil {
		writeBlastError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, b)
}

func writeBlastError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrBlastNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrBlastNotSending):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrBlastInvalid),
		errors.Is(err, cc.ErrBlastNoRecipients),
		errors.Is(err, cc.ErrBlastTestAddress),
		errors.Is(err, cc.ErrBlastEmailUnavailable),
		errors.Is(err, cc.ErrBlastSMSUnavailable):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	// Not a 5xx: those messages are replaced with a generic one, and the
	// admin needs to know the test didn't go out (same as contact replies).
	case errors.Is(err, cc.ErrBlastTestSendFailed):
		loggers.GetCommonError(ctx, err.Error(), http.StatusFailedDependency)
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
