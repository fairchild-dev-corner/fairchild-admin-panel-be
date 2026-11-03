package handlers

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	cc "fairchild-admin-panel-be/internal/constants"
	middlewares "fairchild-admin-panel-be/internal/middlewares"
	models "fairchild-admin-panel-be/internal/models/members"
	service "fairchild-admin-panel-be/internal/services/members"
	"fairchild-admin-panel-be/internal/utils"
	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
)

// handleListMembers: GET /admin/members?category=&branches=&q=&page=&pageSize=
func (h *MembersHandler) handleListMembers(ctx *gin.Context) {
	q, ok := listQuery(ctx)
	if !ok {
		return
	}

	page, err := h.membersService.ListMembers(ctx, q)
	if err != nil {
		writeMemberError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, page)
}

// listQuery reads the list filters, answering 400 itself for an unknown
// category or branch. category defaults to regular, branches ("main,tiic")
// to all, page to 1; pageSize is clamped to 1..MaxPageSize.
func listQuery(ctx *gin.Context) (models.MemberListQuery, bool) {
	q := models.MemberListQuery{
		Category: ctx.DefaultQuery("category", "regular"),
		Search:   ctx.Query("q"),
		Page:     max(1, atoiOr(ctx.Query("page"), 1)),
		PageSize: min(max(1, atoiOr(ctx.Query("pageSize"), service.DefaultPageSize)), service.MaxPageSize),
	}
	if !slices.Contains(models.Categories, q.Category) {
		loggers.GetCommonError(ctx, cc.INVALID_MEMBER_FILTER, http.StatusBadRequest)
		return q, false
	}
	for _, b := range strings.Split(ctx.Query("branches"), ",") {
		if b = strings.TrimSpace(b); b == "" || slices.Contains(q.Branches, b) {
			continue
		}
		if !slices.Contains(models.Branches, b) {
			loggers.GetCommonError(ctx, cc.INVALID_MEMBER_FILTER, http.StatusBadRequest)
			return q, false
		}
		q.Branches = append(q.Branches, b)
	}
	return q, true
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

// handleGetMember: GET /admin/members/:id
func (h *MembersHandler) handleGetMember(ctx *gin.Context) {
	member, err := h.membersService.GetMember(ctx, ctx.Param("id"))
	if err != nil {
		writeMemberError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, member)
}

// handleCreateMember: POST /admin/members
func (h *MembersHandler) handleCreateMember(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.MemberRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	member, err := h.membersService.CreateMember(ctx, &req, actor)
	if err != nil {
		writeMemberError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, member)
}

// handleUpdateMember: PUT /admin/members/:id
func (h *MembersHandler) handleUpdateMember(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	var req models.MemberRequest
	if !utils.ValidatePayload(ctx, &req) {
		return
	}

	member, err := h.membersService.UpdateMember(ctx, ctx.Param("id"), &req, actor)
	if err != nil {
		writeMemberError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, member)
}

// handleDeleteMember: DELETE /admin/members/:id
func (h *MembersHandler) handleDeleteMember(ctx *gin.Context) {
	actor, ok := middlewares.ActorFromContext(ctx)
	if !ok {
		return
	}

	if err := h.membersService.DeleteMember(ctx, ctx.Param("id"), actor); err != nil {
		writeMemberError(ctx, err)
		return
	}
	loggers.SuccessResponse(ctx, nil)
}
