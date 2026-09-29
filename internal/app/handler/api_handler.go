package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"tire-braking/internal/app/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// GetTireConditionsAPI — GET /api/tire-conditions, опциональный ?grip= фильтр по mu_tire
func (h *Handler) GetTireConditionsAPI(ctx *gin.Context) {
	gripStr := ctx.Query("grip")

	var services []repository.TireCondition
	var err error
	if gripStr != "" {
		services, err = h.Repository.SearchByMinTireGrip(parseDecimal(gripStr))
	} else {
		services, err = h.Repository.GetPublished()
	}
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := make([]repository.TireConditionResponse, len(services))
	for i, s := range services {
		response[i] = s.ToResponse()
	}
	ctx.JSON(http.StatusOK, response)
}

// GetTireConditionByIDAPI — GET /api/tire-conditions/:id
func (h *Handler) GetTireConditionByIDAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	tc, err := h.Repository.GetByID(id)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.JSON(http.StatusOK, tc.ToResponse())
}

// PostTireConditionAPI — POST /api/tire-conditions, multipart/form-data
func (h *Handler) PostTireConditionAPI(ctx *gin.Context) {
	name := ctx.PostForm("name")
	if name == "" {
		ctx.Status(http.StatusBadRequest)
		return
	}
	description := ctx.PostForm("description")
	muTire := parseDecimal(ctx.PostForm("mu_tire"))
	muSurface := parseDecimal(ctx.PostForm("mu_surface"))

	imageFile, imageHeader, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	defer imageFile.Close()

	videoFile, videoHeader, err := ctx.Request.FormFile("video")
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	defer videoFile.Close()

	imageURL, err := repository.UploadFile(imageFile, imageHeader, "tire")
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	videoURL, err := repository.UploadFile(videoFile, videoHeader, "tire")
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	tc, err := h.Repository.CreateTireCondition(name, description, imageURL, videoURL, muTire, muSurface, repository.CurrentUser())
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusCreated, tc.ToResponse())
}

// PostRequestItemAPI — POST /api/requests/items, body {"tire_condition_id": число}
func (h *Handler) PostRequestItemAPI(ctx *gin.Context) {
	var body struct {
		TireConditionID uint `json:"tire_condition_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	item, err := h.Repository.AddItemToDraftRequest(repository.CurrentUser(), body.TireConditionID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicateRequestItem):
			ctx.Status(http.StatusConflict)
		case errors.Is(err, repository.ErrTireConditionUnavailable):
			ctx.Status(http.StatusNotFound)
		default:
			ctx.Status(http.StatusInternalServerError)
		}
		return
	}

	ctx.JSON(http.StatusCreated, item.ToResponse())
}

// PutRequestItemAPI — PUT /api/requests/items/:tire_condition_id, body {"position": число}
func (h *Handler) PutRequestItemAPI(ctx *gin.Context) {
	tireConditionID, err := strconv.Atoi(ctx.Param("tire_condition_id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var body struct {
		Position int `json:"position"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	req, err := h.Repository.FindDraftRequest(repository.CurrentUser())
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	err = h.Repository.UpdateRequestItemPosition(req.ID, uint(tireConditionID), body.Position)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	item, err := h.Repository.GetRequestItem(req.ID, uint(tireConditionID))
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, item.ToResponse())
}

// DeleteRequestItemAPI — DELETE /api/requests/items/:tire_condition_id
func (h *Handler) DeleteRequestItemAPI(ctx *gin.Context) {
	tireConditionID, err := strconv.Atoi(ctx.Param("tire_condition_id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	req, err := h.Repository.FindDraftRequest(repository.CurrentUser())
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	err = h.Repository.DeleteRequestItem(req.ID, uint(tireConditionID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// PostRegisterAPI — POST /api/register, body {"username": "...", "password": "..."}
func (h *Handler) PostRegisterAPI(ctx *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		ctx.Status(http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			ctx.Status(http.StatusBadRequest)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	user, err := h.Repository.CreateUser(body.Username, string(hash))
	if err != nil {
		if errors.Is(err, repository.ErrUsernameTaken) {
			ctx.Status(http.StatusConflict)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusCreated, user.ToResponse())
}

// PostAuthAPI — POST /api/auth, заглушка под реальную авторизацию в ЛР4
func (h *Handler) PostAuthAPI(ctx *gin.Context) {
	ctx.Status(http.StatusNotImplemented)
}

// PostLogoutAPI — POST /api/logout, заглушка под реальную авторизацию в ЛР4
func (h *Handler) PostLogoutAPI(ctx *gin.Context) {
	ctx.Status(http.StatusNotImplemented)
}

// statusTransitionErrorCode — маппинг ошибок доменных переходов статуса заявки в HTTP-код
func statusTransitionErrorCode(err error) int {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound
	case errors.Is(err, repository.ErrInvalidStatusTransition):
		return http.StatusConflict
	case errors.Is(err, repository.ErrRequestNotReadyToForm):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// GetCartAPI — GET /api/requests/cart
func (h *Handler) GetCartAPI(ctx *gin.Context) {
	requestID, itemsCount, err := h.Repository.GetCartInfo(repository.CurrentUser())
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, repository.CartResponse{RequestID: requestID, ItemsCount: itemsCount})
}

// GetRequestsListAPI — GET /api/requests, ?status=&date_from=&date_to= (даты в формате 2026-01-01)
func (h *Handler) GetRequestsListAPI(ctx *gin.Context) {
	statusFilter := ctx.Query("status")

	var dateFrom, dateTo *time.Time
	if s := ctx.Query("date_from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		dateFrom = &t
	}
	if s := ctx.Query("date_to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		dateTo = &t
	}

	rows, err := h.Repository.GetRequestsList(statusFilter, dateFrom, dateTo)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := make([]repository.RequestListItemResponse, len(rows))
	for i, row := range rows {
		response[i] = row.ToResponse()
	}
	ctx.JSON(http.StatusOK, response)
}

// GetRequestByIDAPI — GET /api/requests/:id
func (h *Handler) GetRequestByIDAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	req, items, err := h.Repository.GetRequestWithItems(uint(id))
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.JSON(http.StatusOK, repository.ToRequestDetailResponse(req, items))
}

// PutRequestSpeedAPI — PUT /api/requests/:id, body {"speed_kmh": число}
func (h *Handler) PutRequestSpeedAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var body struct {
		SpeedKmh int `json:"speed_kmh"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.Repository.UpdateRequestSpeed(uint(id), body.SpeedKmh); err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	req, items, err := h.Repository.GetRequestWithItems(uint(id))
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, repository.ToRequestDetailResponse(req, items))
}

// PutRequestFormAPI — PUT /api/requests/:id/form, draft -> formed
func (h *Handler) PutRequestFormAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.Repository.FormRequest(uint(id)); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	req, items, err := h.Repository.GetRequestWithItems(uint(id))
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, repository.ToRequestDetailResponse(req, items))
}

// PutRequestCompleteAPI — PUT /api/requests/:id/complete, formed -> completed (модератор = CurrentUser())
func (h *Handler) PutRequestCompleteAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.Repository.CompleteRequest(uint(id), repository.CurrentUser()); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	req, items, err := h.Repository.GetRequestWithItems(uint(id))
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, repository.ToRequestDetailResponse(req, items))
}

// PutRequestRejectAPI — PUT /api/requests/:id/reject, formed -> rejected (модератор = CurrentUser())
func (h *Handler) PutRequestRejectAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.Repository.RejectRequest(uint(id), repository.CurrentUser()); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	req, items, err := h.Repository.GetRequestWithItems(uint(id))
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, repository.ToRequestDetailResponse(req, items))
}

// DeleteRequestAPI — DELETE /api/requests/:id, draft -> deleted (логическое удаление)
func (h *Handler) DeleteRequestAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.Repository.DeleteRequest(uint(id)); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	ctx.Status(http.StatusNoContent)
}
