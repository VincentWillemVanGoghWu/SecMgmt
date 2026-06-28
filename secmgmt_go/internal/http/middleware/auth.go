package middleware

import (
	"strings"

	"secmgmt_go/internal/http/response"
	"secmgmt_go/internal/util"

	"github.com/gin-gonic/gin"
)

const ContextUserIDKey = "currentUserID"

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			response.Error(c, 401, "missing authorization header")
			c.Abort()
			return
		}

		tokenValue := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		if tokenValue == header || tokenValue == "" {
			response.Error(c, 401, "invalid authorization header")
			c.Abort()
			return
		}

		claims, err := util.ParseToken(secret, tokenValue)
		if err != nil {
			response.Error(c, 401, "invalid token")
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Next()
	}
}

func CurrentUserID(c *gin.Context) uint {
	value, ok := c.Get(ContextUserIDKey)
	if !ok {
		return 0
	}
	userID, ok := value.(uint)
	if !ok {
		return 0
	}
	return userID
}
