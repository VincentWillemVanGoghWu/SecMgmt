package response

import "github.com/gin-gonic/gin"

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(200, envelope{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, envelope{
		Code:    status,
		Message: message,
		Data:    gin.H{},
	})
}
