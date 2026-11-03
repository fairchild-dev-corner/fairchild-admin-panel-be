package loggers

import (
	"log/slog"
	"net/http"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	common "fairchild-admin-panel-be/internal/models/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

/**
* Response helpers
* @Description: Same envelopes and helper names as fairchild_be. Response
* bodies are deliberately NOT logged - login/refresh bodies carry access
* tokens, and application bodies carry applicants' personal data.
**/

// SuccessResponse writes a 200 wrapped in common.SuccessResponse.
func SuccessResponse(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, &common.SuccessResponse{
		SuccessID:  uuid.NewString(),
		Status:     cc.SUCCESS,
		HttpCode:   http.StatusOK,
		ResponseAt: time.Now(),
		Body:       data,
	})
}

// GetCommonError writes errCode wrapped in common.ErrorResponse. 5xx
// messages are logged server-side and replaced with a generic message, so a
// raw driver/SQL error never reaches the browser.
func GetCommonError(ctx *gin.Context, err string, errCode int) {
	msg := err
	if errCode >= http.StatusInternalServerError {
		slog.Error("request failed",
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"status", errCode,
			"error", err,
		)
		msg = "Something went wrong. Please try again."
	} else {
		slog.Warn("request rejected",
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"status", errCode,
			"error", err,
		)
	}

	ctx.JSON(errCode, &common.ErrorResponse{
		ErrorCode: errCode,
		TimeStamp: time.Now(),
		ErrorMsg:  msg,
	})
}

// ErrorHandler captures errors added via ctx.Error and returns a consistent
// JSON error response, if nothing has been written yet.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 && !c.Writer.Written() {
			GetCommonError(c, c.Errors.Last().Error(), http.StatusInternalServerError)
		}
	}
}
