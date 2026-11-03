package utils

import (
	"net/http"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	common "fairchild-admin-panel-be/internal/models/common"

	"github.com/gin-gonic/gin"
)

// ValidatePayload binds the request body JSON into v and runs struct
// validation tags against it. It writes the error response itself and
// returns false when binding/validation fails - callers must return
// immediately in that case. Unlike fairchild_be's version, failures use the
// standard ErrorResponse envelope so the panel's extractErrorMessage can
// show them.
func ValidatePayload(ctx *gin.Context, v interface{}) bool {
	if err := ctx.ShouldBindJSON(v); err != nil {
		writeBadRequest(ctx, cc.INVALID_PAYLOAD)
		return false
	}

	if err := Validate.Struct(v); err != nil {
		writeBadRequest(ctx, err.Error())
		return false
	}

	return true
}

func writeBadRequest(ctx *gin.Context, msg string) {
	ctx.AbortWithStatusJSON(http.StatusBadRequest, common.ErrorResponse{
		ErrorCode: http.StatusBadRequest,
		TimeStamp: time.Now(),
		ErrorMsg:  msg,
	})
}
