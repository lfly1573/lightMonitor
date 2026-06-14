package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lightmonitor/internal/application/install"
)

type InstallHandler struct {
	service *install.Service
}

func NewInstallHandler(service *install.Service) *InstallHandler {
	return &InstallHandler{service: service}
}

func (h *InstallHandler) Status(c *gin.Context) {
	installed, err := h.service.IsInstalled(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Fail(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, OK(gin.H{"installed": installed}))
}

func (h *InstallHandler) Install(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Fail(400, "invalid request body"))
		return
	}

	if err := h.service.Install(c.Request.Context(), req.Username, req.Password); err != nil {
		switch {
		case errors.Is(err, install.ErrAlreadyInstalled):
			c.JSON(http.StatusConflict, Fail(409, "system already installed"))
		case errors.Is(err, install.ErrInvalidAdmin):
			c.JSON(http.StatusBadRequest, Fail(400, "invalid administrator account"))
		default:
			c.JSON(http.StatusInternalServerError, Fail(500, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, OK(gin.H{"installed": true}))
}
