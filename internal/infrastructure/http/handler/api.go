package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"lightmonitor/internal/application/core"
)

type APIHandler struct {
	service  *core.Service
	sessions *SessionManager
}

func NewAPIHandler(service *core.Service, sessions *SessionManager) *APIHandler {
	return &APIHandler{service: service, sessions: sessions}
}

func (h *APIHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, OK(gin.H{"status": "ok"}))
}

func (h *APIHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	user, err := h.service.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		writeError(c, err)
		return
	}
	h.sessions.Create(c, user)
	c.JSON(http.StatusOK, OK(user))
}

func (h *APIHandler) Logout(c *gin.Context) {
	h.sessions.Destroy(c)
	c.JSON(http.StatusOK, OK(gin.H{"logout": true}))
}

func (h *APIHandler) Me(c *gin.Context) {
	user, _ := CurrentUser(c)
	c.JSON(http.StatusOK, OK(user))
}

func (h *APIHandler) Receive(c *gin.Context) {
	var payload core.PassivePayload
	if !bindJSON(c, &payload) {
		return
	}
	interval, setting, err := h.service.ReceivePassive(c.Request.Context(), payload)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{
		"interval": interval,
		"setting":  setting,
	}))
}

func (h *APIHandler) Dashboard(c *gin.Context) {
	dash, err := h.service.Dashboard(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(dash))
}

func (h *APIHandler) Settings(c *gin.Context) {
	settings, err := h.service.Settings(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(settings))
}

func (h *APIHandler) UpdateSettings(c *gin.Context) {
	var req []core.Setting
	if !bindJSON(c, &req) {
		return
	}
	if err := h.service.UpdateSettings(c.Request.Context(), req); err != nil {
		writeError(c, err)
		return
	}
	h.Settings(c)
}

func (h *APIHandler) Users(c *gin.Context) {
	users, err := h.service.Users(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(users))
}

func (h *APIHandler) CreateUser(c *gin.Context) {
	var req core.UserInput
	if !bindJSON(c, &req) {
		return
	}
	user, err := h.service.CreateUser(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(user))
}

func (h *APIHandler) UpdateUser(c *gin.Context) {
	var req core.UserInput
	if !bindJSON(c, &req) {
		return
	}
	user, err := h.service.UpdateUser(c.Request.Context(), idParam(c, "id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(user))
}

func (h *APIHandler) DeleteUser(c *gin.Context) {
	if err := h.service.DeleteUser(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) Groups(c *gin.Context) {
	groups, err := h.service.Groups(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(groups))
}

func (h *APIHandler) CreateGroup(c *gin.Context) {
	var req core.GroupInput
	if !bindJSON(c, &req) {
		return
	}
	group, err := h.service.CreateGroup(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(group))
}

func (h *APIHandler) UpdateGroup(c *gin.Context) {
	var req core.GroupInput
	if !bindJSON(c, &req) {
		return
	}
	group, err := h.service.UpdateGroup(c.Request.Context(), idParam(c, "id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(group))
}

func (h *APIHandler) DeleteGroup(c *gin.Context) {
	if err := h.service.DeleteGroup(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) Items(c *gin.Context) {
	items, err := h.service.Items(c.Request.Context(), queryInt64(c, "group_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(items))
}

func (h *APIHandler) CreateItem(c *gin.Context) {
	var req core.ItemInput
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.service.CreateItem(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(item))
}

func (h *APIHandler) UpdateItem(c *gin.Context) {
	var req core.ItemInput
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.service.UpdateItem(c.Request.Context(), idParam(c, "id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(item))
}

func (h *APIHandler) DeleteItem(c *gin.Context) {
	if err := h.service.DeleteItem(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) ActiveRequests(c *gin.Context) {
	requests, err := h.service.ActiveRequests(c.Request.Context(), queryInt64(c, "group_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(requests))
}

func (h *APIHandler) CreateActiveRequest(c *gin.Context) {
	var req core.ActiveRequestInput
	if !bindJSON(c, &req) {
		return
	}
	active, err := h.service.CreateActiveRequest(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(active))
}

func (h *APIHandler) UpdateActiveRequest(c *gin.Context) {
	var req core.ActiveRequestInput
	if !bindJSON(c, &req) {
		return
	}
	active, err := h.service.UpdateActiveRequest(c.Request.Context(), idParam(c, "id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(active))
}

func (h *APIHandler) DeleteActiveRequest(c *gin.Context) {
	if err := h.service.DeleteActiveRequest(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) Fields(c *gin.Context) {
	fields, err := h.service.Fields(c.Request.Context(), queryInt64(c, "group_id"), queryInt64(c, "item_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(fields))
}

func (h *APIHandler) UpsertField(c *gin.Context) {
	var req core.FieldInput
	if !bindJSON(c, &req) {
		return
	}
	field, err := h.service.UpsertField(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(field))
}

func (h *APIHandler) DeleteField(c *gin.Context) {
	if err := h.service.DeleteField(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) Channels(c *gin.Context) {
	channels, err := h.service.Channels(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(channels))
}

func (h *APIHandler) UpsertChannel(c *gin.Context) {
	var req core.ChannelInput
	if !bindJSON(c, &req) {
		return
	}
	channel, err := h.service.UpsertChannel(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(channel))
}

func (h *APIHandler) DeleteChannel(c *gin.Context) {
	if err := h.service.DeleteChannel(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) Rules(c *gin.Context) {
	rules, err := h.service.Rules(c.Request.Context(), queryInt64(c, "group_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(rules))
}

func (h *APIHandler) UpsertRule(c *gin.Context) {
	var req core.AlertRuleInput
	if !bindJSON(c, &req) {
		return
	}
	rule, err := h.service.UpsertRule(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(rule))
}

func (h *APIHandler) DeleteRule(c *gin.Context) {
	if err := h.service.DeleteRule(c.Request.Context(), idParam(c, "id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(gin.H{"deleted": true}))
}

func (h *APIHandler) Samples(c *gin.Context) {
	latest := c.Query("latest") == "true" || c.Query("latest") == "1"
	samples, err := h.service.Samples(c.Request.Context(), queryInt64(c, "group_id"), queryInt64(c, "item_id"), queryInt(c, "limit"), latest)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(samples))
}

func (h *APIHandler) Stats(c *gin.Context) {
	since := time.Now().Add(-24 * time.Hour)
	if hours := queryInt(c, "hours"); hours > 0 {
		since = time.Now().Add(-time.Duration(hours) * time.Hour)
	}
	stats, err := h.service.Stats(c.Request.Context(), queryInt64(c, "group_id"), queryInt64(c, "item_id"), c.Query("field_path"), since)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, OK(stats))
}

func (h *APIHandler) Events(c *gin.Context) {
	var since *time.Time
	if sinceHours := queryInt(c, "since_hours"); sinceHours > 0 {
		t := time.Now().Add(-time.Duration(sinceHours) * time.Hour)
		since = &t
	}
	offsetStr := c.Query("offset")
	pageStr := c.Query("page")
	isPaginated := offsetStr != "" || pageStr != ""

	offset := 0
	if offsetStr != "" {
		offset = queryInt(c, "offset")
	} else if pageStr != "" {
		page := queryInt(c, "page")
		limit := queryInt(c, "limit")
		if limit <= 0 {
			limit = 20
		}
		if page > 1 {
			offset = (page - 1) * limit
		}
	}

	events, total, err := h.service.Events(c.Request.Context(), queryInt(c, "limit"), offset, since, queryInt64(c, "group_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if isPaginated {
		c.JSON(http.StatusOK, OK(gin.H{
			"events": events,
			"total":  total,
		}))
	} else {
		c.JSON(http.StatusOK, OK(events))
	}
}

func bindJSON(c *gin.Context, dest interface{}) bool {
	if err := c.ShouldBindJSON(dest); err != nil {
		c.JSON(http.StatusBadRequest, Fail(400, "invalid request body"))
		return false
	}
	return true
}

func writeError(c *gin.Context, err error) {
	switch err {
	case core.ErrUnauthorized:
		c.JSON(http.StatusUnauthorized, Fail(401, "unauthorized"))
	case core.ErrForbidden:
		c.JSON(http.StatusForbidden, Fail(403, "forbidden"))
	case core.ErrInvalidInput:
		c.JSON(http.StatusBadRequest, Fail(400, "invalid input"))
	default:
		c.JSON(http.StatusInternalServerError, Fail(500, err.Error()))
	}
}

func idParam(c *gin.Context, name string) int64 {
	id, _ := strconv.ParseInt(c.Param(name), 10, 64)
	return id
}

func queryInt64(c *gin.Context, name string) int64 {
	id, _ := strconv.ParseInt(c.Query(name), 10, 64)
	return id
}

func queryInt(c *gin.Context, name string) int {
	value, _ := strconv.Atoi(c.Query(name))
	return value
}
