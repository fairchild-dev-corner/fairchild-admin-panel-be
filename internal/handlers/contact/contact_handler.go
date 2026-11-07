package handlers

import (
	"errors"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
	"net/http"
	"strconv"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/contact"
	service "fairchild-admin-panel-be/internal/services/contact"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 15
	maxPageSize     = 100
)

type ContactHandler struct {
	contactService *service.ContactService
}

func NewHandler(contactService *service.ContactService) *ContactHandler {
	return &ContactHandler{contactService: contactService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Replying
// (which emails or texts the sender) is admins only unless allowed for staff
// on the Capabilities page.
func (h *ContactHandler) RegisterRoutes(admin *gin.RouterGroup) {
	need := middlewares.RequireCapability
	admin.GET("/contact-messages", need(caps.ContactView), h.handleList)
	admin.GET("/contact-messages/:id", need(caps.ContactView), h.handleGet)
	admin.POST("/contact-messages/:id/replies", need(caps.ContactReply), h.handleReply)
}

// handleList: GET /admin/contact-messages?status=&q=&page=&pageSize=
func (h *ContactHandler) handleList(ctx *gin.Context) {
	q := models.ContactListQuery{
		Status:   ctx.DefaultQuery("status", "unreplied"),
		Search:   ctx.Query("q"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), defaultPageSize)), maxPageSize),
	}
	if !utils.IsTabFilter(models.Statuses, q.Status) {
		loggers.GetCommonError(ctx, cc.INVALID_CONTACT_FILTER, http.StatusBadRequest)
		return
	}

	page, err := h.contactService.List(ctx, q)
	if err != nil {
		writeContactError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// handleGet: GET /admin/contact-messages/:id
func (h *ContactHandler) handleGet(ctx *gin.Context) {
	m, err := h.contactService.Get(ctx, ctx.Param("id"))
	if err != nil {
		writeContactError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

// handleReply: POST /admin/contact-messages/:id/replies {channel, body}
func (h *ContactHandler) handleReply(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.ReplyRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	m, err := h.contactService.Reply(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeContactError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, m)
}

func writeContactError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrContactMessageNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrReplyEmailUnavailable),
		errors.Is(err, cc.ErrReplySMSUnavailable),
		errors.Is(err, cc.ErrReplyNoMobile),
		errors.Is(err, cc.ErrReplyInvalidEmail),
		errors.Is(err, cc.ErrReplyEmpty),
		errors.Is(err, cc.ErrReplyTooLongForSMS),
		errors.Is(err, cc.ErrInvalidPayload):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	// 4xx rather than 5xx so the message isn't replaced by the generic one:
	// the admin needs to know whether the member received the reply.
	case errors.Is(err, cc.ErrReplySendFailed):
		loggers.GetCommonError(ctx, err.Error(), http.StatusFailedDependency)
	case errors.Is(err, cc.ErrReplySentNotRecorded):
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
