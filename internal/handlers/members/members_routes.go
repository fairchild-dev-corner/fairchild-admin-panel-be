package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	service "fairchild-admin-panel-be/internal/services/members"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type MembersHandler struct {
	membersService *service.MembersService
}

func NewHandler(membersService *service.MembersService) *MembersHandler {
	return &MembersHandler{membersService: membersService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Admin and
// staff can view, add and edit members; deleting one is admin-only. Ledger
// members can only be viewed.
func (h *MembersHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/members", h.handleListMembers)
	admin.POST("/members", h.handleCreateMember)
	admin.GET("/members/:id", h.handleGetMember)
	admin.PUT("/members/:id", h.handleUpdateMember)
	admin.DELETE("/members/:id", middlewares.RequireRole(cc.ROLE_ADMIN), h.handleDeleteMember)
	// Read-only views of a ledger member's accounts.
	admin.GET("/members/:id/soa", h.handleListStatements)
	admin.GET("/members/:id/soa/:ctrlNo", h.handleGetStatement)
	admin.GET("/members/:id/standing", h.handleGetStanding)
}

func writeMemberError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrMemberNotFound),
		errors.Is(err, cc.ErrStatementNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrLedgerMemberReadOnly),
		errors.Is(err, cc.ErrNotLedgerMember):
		loggers.GetCommonError(ctx, err.Error(), http.StatusForbidden)
	case errors.Is(err, cc.ErrInvalidPayload),
		errors.Is(err, cc.ErrDateOfBirthInFuture),
		errors.Is(err, cc.ErrJoinedAtInFuture),
		errors.Is(err, cc.ErrInvalidMobileNumber),
		errors.Is(err, cc.ErrMobileNumberRequired),
		errors.Is(err, cc.ErrGuardianRequired):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}
