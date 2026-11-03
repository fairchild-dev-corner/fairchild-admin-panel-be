package handlers

import (
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/applications"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

// handleListLoanApplications: GET /admin/loan-applications?status=
func (h *ApplicationsHandler) handleListLoanApplications(ctx *gin.Context) {
	status, ok := statusFilter(ctx)
	if !ok {
		return
	}

	apps, err := h.applicationsService.ListLoanApplications(ctx, status)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, apps)
}

// handleGetLoanApplication: GET /admin/loan-applications/:id
func (h *ApplicationsHandler) handleGetLoanApplication(ctx *gin.Context) {
	app, err := h.applicationsService.GetLoanApplication(ctx, ctx.Param("id"))
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, app)
}

// handleReviewLoanApplication: PUT /admin/loan-applications/:id/review
func (h *ApplicationsHandler) handleReviewLoanApplication(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.ReviewDecisionRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	app, err := h.applicationsService.ReviewLoanApplication(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, app)
}

// handleListMembershipApplications: GET /admin/membership-applications?status=&q=
func (h *ApplicationsHandler) handleListMembershipApplications(ctx *gin.Context) {
	status, ok := statusFilter(ctx)
	if !ok {
		return
	}

	apps, err := h.applicationsService.ListMembershipApplications(ctx, status, ctx.Query("q"))
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, apps)
}

// handleGetMembershipApplication: GET /admin/membership-applications/:id
func (h *ApplicationsHandler) handleGetMembershipApplication(ctx *gin.Context) {
	app, err := h.applicationsService.GetMembershipApplication(ctx, ctx.Param("id"))
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, app)
}

// handleCreateMembershipApplication: POST /admin/membership-applications
func (h *ApplicationsHandler) handleCreateMembershipApplication(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.NewMembershipApplicationRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	app, err := h.applicationsService.CreateMembershipApplication(ctx, &req, actor)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, app)
}

// handleReviewMembershipApplication: PUT /admin/membership-applications/:id/review
func (h *ApplicationsHandler) handleReviewMembershipApplication(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.ReviewDecisionRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	app, err := h.applicationsService.ReviewMembershipApplication(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, app)
}
