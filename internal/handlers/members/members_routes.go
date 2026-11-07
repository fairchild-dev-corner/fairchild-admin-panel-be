package handlers

import (
	"errors"
	caps "fairchild-admin-panel-be/internal/models/capabilities"
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

// RegisterRoutes mounts onto the already-guarded /admin group. What staff
// may see and do here is set on the Capabilities page. Ledger members can
// only be viewed.
func (h *MembersHandler) RegisterRoutes(admin *gin.RouterGroup) {
	need := middlewares.RequireCapability
	admin.GET("/members", need(caps.MembersView), h.handleListMembers)
	admin.POST("/members", need(caps.MembersEdit), h.handleCreateMember)
	admin.GET("/members/:id", need(caps.MembersView), h.handleGetMember)
	admin.PUT("/members/:id", need(caps.MembersEdit), h.handleUpdateMember)
	admin.DELETE("/members/:id", need(caps.MembersDelete), h.handleDeleteMember)
	// Read-only views of a ledger member's accounts.
	admin.GET("/members/:id/soa", need(caps.MembersView), h.handleListStatements)
	admin.GET("/members/:id/soa/:ctrlNo", need(caps.MembersView), h.handleGetStatement)
	admin.GET("/members/:id/standing", need(caps.MembersView), h.handleGetStanding)
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
