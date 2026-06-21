package handler

import (
	"errors"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/http/middleware"
	"secmgmt_go/internal/http/response"
	"secmgmt_go/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var payload dto.LoginRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, 400, "invalid login payload")
		return
	}

	data, err := h.authService.Login(payload)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Error(c, 401, err.Error())
			return
		}
		response.Error(c, 500, err.Error())
		return
	}

	response.OK(c, data)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	response.OK(c, gin.H{})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	data, err := h.authService.GetMe(userID)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *AuthHandler) Menus(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	data, err := h.authService.GetMenus(userID)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}
