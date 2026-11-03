package handlers

import (
	"errors"
	"net/http"

	cc "fairchild-admin-panel-be/internal/constants"
	service "fairchild-admin-panel-be/internal/services/applications"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

type ApplicationsHandler struct {
	applicationsService *service.ApplicationsService
}

func NewHandler(applicationsService *service.ApplicationsService) *ApplicationsHandler {
	return &ApplicationsHandler{applicationsService: applicationsService}
}

// RegisterRoutes mounts onto the already-guarded /admin group. Both admin
// and staff may review applications.
func (h *ApplicationsHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/loan-applications", h.handleListLoanApplications)
	admin.GET("/loan-applications/:id", h.handleGetLoanApplication)
	admin.PUT("/loan-applications/:id/review", h.handleReviewLoanApplication)

	admin.GET("/membership-applications", h.handleListMembershipApplications)
	admin.POST("/membership-applications", h.handleCreateMembershipApplication)
	admin.GET("/membership-applications/:id", h.handleGetMembershipApplication)
	admin.PUT("/membership-applications/:id/review", h.handleReviewMembershipApplication)
}

// statusFilter reads ?status=, answering 400 itself for an unknown value.
// "" and "all" both mean no filter.
func statusFilter(ctx *gin.Context) (string, bool) {
	switch status := ctx.Query("status"); status {
	case "", "all":
		return "", true
	case cc.APPLICATION_STATUS_PENDING, cc.APPLICATION_STATUS_APPROVED, cc.APPLICATION_STATUS_REJECTED:
		return status, true
	default:
		loggers.GetCommonError(ctx, cc.INVALID_STATUS_FILTER, http.StatusBadRequest)
		return "", false
	}
}

func writeApplicationError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, cc.ErrApplicationNotFound):
		loggers.GetCommonError(ctx, err.Error(), http.StatusNotFound)
	case errors.Is(err, cc.ErrApplicationAlreadyReviewed):
		loggers.GetCommonError(ctx, err.Error(), http.StatusConflict)
	case errors.Is(err, cc.ErrInvalidPayload),
		errors.Is(err, cc.ErrDateOfBirthInFuture),
		errors.Is(err, cc.ErrInvalidMobileNumber),
		errors.Is(err, cc.ErrCoopApplicantIncomplete):
		loggers.GetCommonError(ctx, err.Error(), http.StatusBadRequest)
	default:
		loggers.GetCommonError(ctx, err.Error(), http.StatusInternalServerError)
	}
}
