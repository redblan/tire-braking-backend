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
//
// @Summary     Список опубликованных услуг
// @Description Для гостя is_owner = false
// @Tags        tire-conditions
// @Produce     json
// @Param       grip query number false "Минимальный коэффициент сцепления шины (mu_tire)"
// @Success     200 {array} repository.TireConditionResponse
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions [get]
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
	for i := range response {
		response[i].IsOwner = isOwner(ctx, services[i].CreatorID)
	}
	ctx.JSON(http.StatusOK, response)
}

// GetTireConditionsFeedAPI — GET /api/tire-conditions/feed, все опубликованные услуги без фильтра
//
// @Summary     Лента опубликованных услуг
// @Description Для гостя is_owner = false
// @Tags        tire-conditions
// @Produce     json
// @Success     200 {array} repository.TireConditionResponse
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/feed [get]
func (h *Handler) GetTireConditionsFeedAPI(ctx *gin.Context) {
	services, err := h.Repository.GetPublished()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := make([]repository.TireConditionResponse, len(services))
	for i, s := range services {
		response[i] = s.ToResponse()
		response[i].IsOwner = isOwner(ctx, s.CreatorID)
	}
	ctx.JSON(http.StatusOK, response)
}

// GetMyDraftTireConditionAPI — GET /api/tire-conditions/draft, черновик услуги текущего пользователя
//
// @Summary     Черновик услуги текущего пользователя
// @Tags        tire-conditions
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} repository.TireConditionResponse
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Черновика нет"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/draft [get]
func (h *Handler) GetMyDraftTireConditionAPI(ctx *gin.Context) {
	userID, _ := currentUserID(ctx)
	tc, err := h.Repository.FindMyDraftTireCondition(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := tc.ToResponse()
	response.IsOwner = true
	ctx.JSON(http.StatusOK, response)
}

// GetTireConditionByIDAPI — GET /api/tire-conditions/:id
//
// @Summary     Услуга по id
// @Description Для гостя is_owner = false
// @Tags        tire-conditions
// @Produce     json
// @Param       id path int true "ID услуги"
// @Success     200 {object} repository.TireConditionResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Не найдено"
// @Router      /api/tire-conditions/{id} [get]
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

	response := tc.ToResponse()
	response.IsOwner = isOwner(ctx, tc.CreatorID)
	ctx.JSON(http.StatusOK, response)
}

// PostTireConditionAPI — POST /api/tire-conditions, multipart/form-data
//
// @Summary     Создать услугу (черновик) с загрузкой изображения и видео в Minio
// @Tags        tire-conditions
// @Accept      multipart/form-data
// @Produce     json
// @Security    BearerAuth
// @Param       name        formData string true  "Название"
// @Param       description formData string false "Описание"
// @Param       mu_tire     formData number false "Коэффициент сцепления шины"
// @Param       mu_surface  formData number false "Коэффициент сцепления покрытия"
// @Param       image       formData file   true  "Изображение"
// @Param       video       formData file   true  "Видео"
// @Success     201 {object} repository.TireConditionResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions [post]
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

	userID, _ := currentUserID(ctx)
	tc, err := h.Repository.CreateTireCondition(name, description, imageURL, videoURL, muTire, muSurface, userID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusCreated, tc.ToResponse())
}

// PutPublishTireConditionAPI — PUT /api/tire-conditions/:id/publish, body {"description": "...", "mu_tire": число, "mu_surface": число}
//
// @Summary     Опубликовать черновик услуги
// @Description Своя услуга; модератор — любая
// @Tags        tire-conditions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path int true "ID услуги"
// @Param       body body PublishTireConditionRequest true "Описание и коэффициенты (> 0)"
// @Success     200 {object} repository.TireConditionResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Нет прав / чужой объект"
// @Failure     404 "Не найдено"
// @Failure     409 "Недопустимый переход статуса"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/{id}/publish [put]
func (h *Handler) PutPublishTireConditionAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var body PublishTireConditionRequest
	// коэффициенты сцепления стоят в знаменателе формулы тормозного пути, поэтому должны быть > 0
	if err := ctx.ShouldBindJSON(&body); err != nil || body.MuTire <= 0 || body.MuSurface <= 0 {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	if err := h.Repository.PublishTireCondition(uint(id), body.Description, body.MuTire, body.MuSurface, userID, isModerator(ctx)); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	tc, err := h.Repository.GetByID(id)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := tc.ToResponse()
	response.IsOwner = isOwner(ctx, tc.CreatorID)
	ctx.JSON(http.StatusOK, response)
}

// DeleteTireConditionAPI — DELETE /api/tire-conditions/:id, логическое удаление; своей услуги, модератору — любой
//
// @Summary     Удалить услугу (логически)
// @Description Своя услуга; модератор — любая
// @Tags        tire-conditions
// @Security    BearerAuth
// @Param       id path int true "ID услуги"
// @Success     204 "Удалено"
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Нет прав / чужой объект"
// @Failure     404 "Не найдено"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/{id} [delete]
func (h *Handler) DeleteTireConditionAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	err = h.Repository.DeleteTireCondition(uint(id), userID, isModerator(ctx))
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotOwner):
			ctx.Status(http.StatusForbidden)
		case errors.Is(err, gorm.ErrRecordNotFound):
			ctx.Status(http.StatusNotFound)
		default:
			ctx.Status(http.StatusInternalServerError)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// PostLikeTireConditionAPI — POST /api/tire-conditions/:id/like, body {"value": 0 или 1}
//
// @Summary     Поставить или снять лайк
// @Tags        tire-conditions
// @Accept      json
// @Security    BearerAuth
// @Param       id   path int true "ID услуги"
// @Param       body body LikeRequest true "value: 1 — поставить, 0 — снять"
// @Success     204 "Готово"
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Не найдено"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/{id}/like [post]
func (h *Handler) PostLikeTireConditionAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var body LikeRequest
	if err := ctx.ShouldBindJSON(&body); err != nil || body.Value == nil || (*body.Value != 0 && *body.Value != 1) {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	err = h.Repository.ToggleLike(uint(id), userID, *body.Value)
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

// PostRequestItemAPI — POST /api/requests/items, body {"tire_condition_id": число}
//
// @Summary     Добавить услугу в корзину (черновик заявки)
// @Description Черновик заявки создаётся автоматически, если его нет
// @Tags        request-items
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body AddRequestItemRequest true "ID услуги"
// @Success     201 {object} repository.RequestItemResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Услуга не найдена или не опубликована"
// @Failure     409 "Услуга уже в заявке"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/items [post]
func (h *Handler) PostRequestItemAPI(ctx *gin.Context) {
	var body AddRequestItemRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	item, err := h.Repository.AddItemToDraftRequest(userID, body.TireConditionID)
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
//
// @Summary     Изменить позицию услуги в черновике заявки
// @Tags        request-items
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       tire_condition_id path int true "ID услуги"
// @Param       body body RequestItemPositionRequest true "Новая позиция"
// @Success     200 {object} repository.RequestItemResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Нет черновика или услуги в нём"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/items/{tire_condition_id} [put]
func (h *Handler) PutRequestItemAPI(ctx *gin.Context) {
	tireConditionID, err := strconv.Atoi(ctx.Param("tire_condition_id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var body RequestItemPositionRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	req, err := h.Repository.FindDraftRequest(userID)
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
//
// @Summary     Удалить услугу из черновика заявки
// @Tags        request-items
// @Security    BearerAuth
// @Param       tire_condition_id path int true "ID услуги"
// @Success     204 "Удалено"
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Нет черновика или услуги в нём"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/items/{tire_condition_id} [delete]
func (h *Handler) DeleteRequestItemAPI(ctx *gin.Context) {
	tireConditionID, err := strconv.Atoi(ctx.Param("tire_condition_id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	req, err := h.Repository.FindDraftRequest(userID)
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
//
// @Summary     Регистрация
// @Description is_moderator всегда false; пароль хранится как bcrypt-хеш
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body CredentialsRequest true "Логин и пароль"
// @Success     201 {object} repository.UserResponse
// @Failure     400 "Некорректные параметры"
// @Failure     409 "Логин занят"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/register [post]
func (h *Handler) PostRegisterAPI(ctx *gin.Context) {
	var body CredentialsRequest
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

// AuthResponse — DTO для POST /api/auth
type AuthResponse struct {
	AccessToken string `json:"access_token"`
}

// PostAuthAPI — POST /api/auth, body {"username": "...", "password": "..."};
// неверный логин или пароль -> 401 без уточнения, что именно неверно
//
// @Summary     Вход, выдача JWT
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body CredentialsRequest true "Логин и пароль"
// @Success     200 {object} AuthResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Неверный логин или пароль"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/auth [post]
func (h *Handler) PostAuthAPI(ctx *gin.Context) {
	var body CredentialsRequest
	if err := ctx.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		ctx.Status(http.StatusBadRequest)
		return
	}

	user, err := h.Repository.GetUserByUsername(body.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Status(http.StatusUnauthorized)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		ctx.Status(http.StatusUnauthorized)
		return
	}

	token, err := h.JWT.GenerateToken(user.ID, user.IsModerator)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, AuthResponse{AccessToken: token})
}

// PostLogoutAPI — POST /api/logout; токен из заголовка попадает в blacklist до своего истечения
//
// @Summary     Выход: токен попадает в blacklist до своего истечения
// @Tags        auth
// @Security    BearerAuth
// @Success     204 "Готово"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/logout [post]
func (h *Handler) PostLogoutAPI(ctx *gin.Context) {
	token, _, ok := bearerToken(ctx)
	if !ok {
		ctx.Status(http.StatusUnauthorized)
		return
	}

	claims, err := h.JWT.ParseToken(token)
	if err != nil {
		ctx.Status(http.StatusUnauthorized)
		return
	}

	if err := h.Blacklist.AddToBlacklist(ctx.Request.Context(), token, time.Until(claims.ExpiresAt.Time)); err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// statusTransitionErrorCode — маппинг ошибок доменных переходов статуса заявки в HTTP-код
func statusTransitionErrorCode(err error) int {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound
	case errors.Is(err, repository.ErrNotOwner):
		return http.StatusForbidden
	case errors.Is(err, repository.ErrInvalidStatusTransition):
		return http.StatusConflict
	case errors.Is(err, repository.ErrRequestNotReadyToForm):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// checkRequestAccess — обычный пользователь работает только со своей заявкой (чужая -> 403, нет -> 404);
// allowModerator — модератору доступна любая. false — ответ уже записан в ctx
func (h *Handler) checkRequestAccess(ctx *gin.Context, id uint, allowModerator bool) bool {
	if allowModerator && isModerator(ctx) {
		return true
	}

	userID, _ := currentUserID(ctx)
	if err := h.Repository.CheckRequestOwner(id, userID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			ctx.Status(http.StatusNotFound)
		case errors.Is(err, repository.ErrNotOwner):
			ctx.Status(http.StatusForbidden)
		default:
			ctx.Status(http.StatusInternalServerError)
		}
		return false
	}
	return true
}

// GetCartAPI — GET /api/requests/cart
//
// @Summary     Корзина: id черновика заявки и число услуг в нём
// @Description request_id = null, если черновика нет
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} repository.CartResponse
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/cart [get]
func (h *Handler) GetCartAPI(ctx *gin.Context) {
	userID, _ := currentUserID(ctx)
	requestID, itemsCount, err := h.Repository.GetCartInfo(userID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, repository.CartResponse{RequestID: requestID, ItemsCount: itemsCount})
}

// GetRequestsListAPI — GET /api/requests, ?status=&date_from=&date_to= (даты в формате 2026-01-01);
// пользователь видит только свои заявки, модератор — все
//
// @Summary     Список заявок (без черновиков и удалённых)
// @Description Пользователь видит свои заявки, модератор — все
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       status    query string false "Статус: formed, completed, rejected"
// @Param       date_from query string false "Дата формирования от, 2006-01-02"
// @Param       date_to   query string false "Дата формирования до, 2006-01-02"
// @Success     200 {array} repository.RequestListItemResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests [get]
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

	var creatorID *uint
	if !isModerator(ctx) {
		userID, _ := currentUserID(ctx)
		creatorID = &userID
	}

	rows, err := h.Repository.GetRequestsList(statusFilter, dateFrom, dateTo, creatorID)
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
//
// @Summary     Заявка с составом
// @Description Своя заявка; модератор — любая
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "ID заявки"
// @Success     200 {object} repository.RequestDetailResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Нет прав / чужой объект"
// @Failure     404 "Не найдено"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/{id} [get]
func (h *Handler) GetRequestByIDAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if !h.checkRequestAccess(ctx, uint(id), true) {
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
//
// @Summary     Задать скорость в заявке
// @Description Только своя заявка
// @Tags        requests
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path int true "ID заявки"
// @Param       body body RequestSpeedRequest true "Скорость, км/ч"
// @Success     200 {object} repository.RequestDetailResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Нет прав / чужой объект"
// @Failure     404 "Не найдено"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/{id} [put]
func (h *Handler) PutRequestSpeedAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if !h.checkRequestAccess(ctx, uint(id), false) {
		return
	}

	var body RequestSpeedRequest
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
//
// @Summary     Сформировать заявку (draft -> formed), расчёт тормозного пути
// @Description Только своя заявка
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "ID заявки"
// @Success     200 {object} repository.RequestDetailResponse
// @Failure     400 "Не задана скорость или пустой состав"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Нет прав / чужой объект"
// @Failure     404 "Не найдено"
// @Failure     409 "Недопустимый переход статуса"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/{id}/form [put]
func (h *Handler) PutRequestFormAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if !h.checkRequestAccess(ctx, uint(id), false) {
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

// PutRequestCompleteAPI — PUT /api/requests/:id/complete, formed -> completed (moderator_id из токена)
//
// @Summary     Завершить заявку (formed -> completed)
// @Description Только модератор; moderator_id берётся из токена
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "ID заявки"
// @Success     200 {object} repository.RequestDetailResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Не модератор"
// @Failure     404 "Не найдено"
// @Failure     409 "Недопустимый переход статуса"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/{id}/complete [put]
func (h *Handler) PutRequestCompleteAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	moderatorID, _ := currentUserID(ctx)
	if err := h.Repository.CompleteRequest(uint(id), moderatorID); err != nil {
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

// PutRequestRejectAPI — PUT /api/requests/:id/reject, formed -> rejected (moderator_id из токена)
//
// @Summary     Отклонить заявку (formed -> rejected)
// @Description Только модератор; moderator_id берётся из токена
// @Tags        requests
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "ID заявки"
// @Success     200 {object} repository.RequestDetailResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Не модератор"
// @Failure     404 "Не найдено"
// @Failure     409 "Недопустимый переход статуса"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/{id}/reject [put]
func (h *Handler) PutRequestRejectAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	moderatorID, _ := currentUserID(ctx)
	if err := h.Repository.RejectRequest(uint(id), moderatorID); err != nil {
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
//
// @Summary     Удалить черновик заявки (логически)
// @Description Только своя заявка
// @Tags        requests
// @Security    BearerAuth
// @Param       id path int true "ID заявки"
// @Success     204 "Удалено"
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     403 "Нет прав / чужой объект"
// @Failure     404 "Не найдено"
// @Failure     409 "Недопустимый переход статуса"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/requests/{id} [delete]
func (h *Handler) DeleteRequestAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if !h.checkRequestAccess(ctx, uint(id), false) {
		return
	}

	if err := h.Repository.DeleteRequest(uint(id)); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	ctx.Status(http.StatusNoContent)
}
