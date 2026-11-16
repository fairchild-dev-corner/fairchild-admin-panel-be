package handlers

import (
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/applications"
	user_models "fairchild-admin-panel-be/internal/models/users"
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

// handleMessageLoanApplicant: POST /admin/loan-applications/:id/messages
// {channel, subject, body} - texts or emails the applicant.
func (h *ApplicationsHandler) handleMessageLoanApplicant(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.MessageApplicantRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	result, err := h.applicationsService.MessageLoanApplicant(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, result)
}

// handleAddLoanApplicationNote: POST /admin/loan-applications/:id/notes
// {body} - adds a staff note.
func (h *ApplicationsHandler) handleAddLoanApplicationNote(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.AddLoanNoteRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	note, err := h.applicationsService.AddLoanApplicationNote(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, note)
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

// handleEditLoanApplicationNote: PUT /admin/loan-applications/:id/notes/:noteId
// {body} - the author rewrites their note.
func (h *ApplicationsHandler) handleEditLoanApplicationNote(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.AddLoanNoteRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	note, err := h.applicationsService.EditLoanApplicationNote(ctx, ctx.Param("id"), ctx.Param("noteId"), &req, actor)
	if err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, note)
}

// handleDeleteLoanApplicationNote: DELETE /admin/loan-applications/:id/notes/:noteId
// - by the note's author or an admin.
func (h *ApplicationsHandler) handleDeleteLoanApplicationNote(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}
	user, _ := middlewares.PanelUserFromContext(ctx)

	if err := h.applicationsService.DeleteLoanApplicationNote(ctx, ctx.Param("id"), ctx.Param("noteId"), actor, user_models.IsAdmin(user.Role)); err != nil {
		writeApplicationError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, gin.H{"deleted": true})
}
