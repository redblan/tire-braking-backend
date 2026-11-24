package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"tire-braking/internal/app/feed"
	"tire-braking/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// listDefaultLimit и listMaxLimit — размер страницы списка по умолчанию и верхняя граница limit
const (
	listDefaultLimit = 10
	listMaxLimit     = 100
)

// GetTireConditionsAPI — GET /api/tire-conditions, страница списка с опциональным ?grip= фильтром по mu_tire
//
// @Summary     Список опубликованных услуг (страница)
// @Description Пагинация: page (с 1) и limit (по умолчанию 10, максимум 100). total — число услуг, подходящих под фильтр. is_owner: 0 или 1 — создатель услуги совпадает с текущим пользователем (для гостя 0)
// @Tags        tire-conditions
// @Produce     json
// @Param       grip  query number false "Минимальный коэффициент сцепления шины (mu_tire)"
// @Param       page  query int    false "Номер страницы, с 1" default(1)
// @Param       limit query int    false "Размер страницы, максимум 100" default(10)
// @Success     200 {object} repository.TireConditionsPage
// @Failure     400 "Некорректные page или limit"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions [get]
func (h *Handler) GetTireConditionsAPI(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		ctx.Status(http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", strconv.Itoa(listDefaultLimit)))
	if err != nil || limit < 1 {
		ctx.Status(http.StatusBadRequest)
		return
	}
	if limit > listMaxLimit {
		limit = listMaxLimit
	}
	offset := (page - 1) * limit

	gripStr := ctx.Query("grip")
	services, total, err := h.Repository.GetPublishedPage(parseDecimal(gripStr), gripStr != "", limit, offset)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	data := make([]repository.TireConditionResponse, len(services))
	for i, s := range services {
		data[i] = s.ToResponse()
		data[i].IsOwner = ownerFlag(ctx, s.CreatorID)
	}
	ctx.JSON(http.StatusOK, repository.TireConditionsPage{Total: total, Page: page, Limit: limit, Data: data})
}

// GetTireConditionsFeedAPI — GET /api/tire-conditions/feed, одна услуга ленты сессии из Redis
//
// @Summary     Одна услуга ленты
// @Description При next=true запускается асинхронный сервис: через 5–10 секунд он присылает новые углы поворота 3D модели этой услуги (angle_x, angle_y), их видно повторным запросом по id. Лента формируется в Redis при первом обращении сессии: 100 случайных неповторяющихся опубликованных услуг с порядковыми номерами (position); гость — по куке feed_session, авторизованный — по user_id. Без id — первая услуга ленты; ?id=N — услуга N; ?id=N&next=true — следующая в ленте после N; после 100-й формируются новые 100. is_liked: 0 или 1 — стоит ли лайк текущего пользователя на услуге (для гостя 0). position — порядковый номер услуги в ленте сессии (0, если услуги нет в ленте).
// @Tags        tire-conditions
// @Produce     json
// @Param       id   query int  false "ID услуги; без параметра — первая услуга ленты"
// @Param       next query bool false "true — вернуть следующую услугу ленты после id"
// @Success     200 {object} repository.FeedResponse
// @Failure     400 "Некорректные параметры"
// @Failure     401 "Нет токена, токен невалиден, просрочен или отозван"
// @Failure     404 "Опубликованных услуг нет или услуга не найдена"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/feed [get]
func (h *Handler) GetTireConditionsFeedAPI(ctx *gin.Context) {
	id := 0
	if idStr := ctx.Query("id"); idStr != "" {
		var err error
		id, err = strconv.Atoi(idStr)
		if err != nil || id < 0 {
			ctx.Status(http.StatusBadRequest)
			return
		}
	}
	next := ctx.Query("next") == "true"

	session := feedSession(ctx)
	reqCtx := ctx.Request.Context()

	ids, err := h.Feed.Ensure(reqCtx, session)
	if err != nil {
		feedError(ctx, err)
		return
	}

	// какую услугу отдать: первую ленты, указанную по id или следующую после id
	targetID := ids[0]
	if id != 0 {
		_, idx := feed.Position(ids, uint(id))
		switch {
		case !next:
			targetID = uint(id)
		case idx == len(ids)-1:
			// просмотрены все карточки ленты — формируются новые 100
			if ids, err = h.Feed.Rebuild(reqCtx, session); err != nil {
				feedError(ctx, err)
				return
			}
			targetID = ids[0]
		case idx >= 0:
			targetID = ids[idx+1]
		}
	}

	tc, err := h.Repository.GetFeedItem(targetID, false)
	if errors.Is(err, gorm.ErrRecordNotFound) && next {
		// услугу ленты успели удалить — формируем ленту заново
		if ids, err = h.Feed.Rebuild(reqCtx, session); err != nil {
			feedError(ctx, err)
			return
		}
		tc, err = h.Repository.GetFeedItem(ids[0], false)
	}
	if err != nil {
		feedError(ctx, err)
		return
	}

	if next {
		// отложенное действие: асинхронный сервис через 5–10 секунд пришлёт новые углы 3D модели этой услуги
		go func(id uint, angleX, angleY float64) {
			if err := h.Async.RequestRotation(id, angleX, angleY); err != nil {
				logrus.Errorf("асинхронный сервис недоступен: %v", err)
			}
		}(tc.ID, tc.AngleX, tc.AngleY)
	}

	position, _ := feed.Position(ids, tc.ID)
	response := repository.FeedResponse{TireConditionResponse: tc.ToResponse(), Position: position}
	response.IsOwner = ownerFlag(ctx, tc.CreatorID)
	if userID, ok := currentUserID(ctx); ok {
		liked, err := h.Repository.IsLiked(tc.ID, userID)
		if err != nil {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		if liked {
			response.IsLiked = 1
		}
	}
	ctx.JSON(http.StatusOK, response)
}

// feedError — 404, если услуги нет (нет опубликованных или услуга не найдена), иначе 500
func feedError(ctx *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, feed.ErrEmpty) {
		ctx.Status(http.StatusNotFound)
		return
	}
	ctx.Status(http.StatusInternalServerError)
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
	response.IsOwner = 1
	ctx.JSON(http.StatusOK, response)
}

// PostTireConditionAPI — POST /api/tire-conditions, multipart/form-data
//
// @Summary     Создать услугу (черновик) с загрузкой изображения и 3D модели в Minio
// @Tags        tire-conditions
// @Accept      multipart/form-data
// @Produce     json
// @Security    BearerAuth
// @Param       name        formData string true  "Название"
// @Param       description formData string false "Описание"
// @Param       mu_tire     formData number false "Коэффициент сцепления шины"
// @Param       mu_surface  formData number false "Коэффициент сцепления покрытия"
// @Param       image       formData file   true  "Изображение"
// @Param       model       formData file   true  "3D модель (.glb)"
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

	modelFile, modelHeader, err := ctx.Request.FormFile("model")
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	defer modelFile.Close()

	imageURL, err := repository.UploadFile(imageFile, imageHeader, "tire")
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	modelURL, err := repository.UploadFile(modelFile, modelHeader, "tire")
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	userID, _ := currentUserID(ctx)
	tc, err := h.Repository.CreateTireCondition(name, description, imageURL, modelURL, muTire, muSurface, userID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusCreated, tc.ToResponse())
}

// PutPublishTireConditionAPI — PUT /api/tire-conditions/:id/publish,
// body {"description": "...", "description_en": "...", "mu_tire": число, "mu_surface": число}
//
// @Summary     Опубликовать черновик услуги
// @Description Своя услуга; модератор — любая
// @Tags        tire-conditions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path int true "ID услуги"
// @Param       body body PublishTireConditionRequest true "Описание, краткое описание на английском (50–100 символов) и коэффициенты (> 0)"
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

	if !validDescriptionEn(body.DescriptionEn) {
		ctx.Status(http.StatusBadRequest)
		return
	}

	userID, _ := currentUserID(ctx)
	if err := h.Repository.PublishTireCondition(uint(id), body.Description, body.DescriptionEn, body.MuTire, body.MuSurface, userID, isModerator(ctx)); err != nil {
		ctx.Status(statusTransitionErrorCode(err))
		return
	}

	tc, err := h.Repository.GetByID(id)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := tc.ToResponse()
	response.IsOwner = ownerFlag(ctx, tc.CreatorID)
	ctx.JSON(http.StatusOK, response)
}

// validDescriptionEn — краткое описание на английском: пустое или 50–100 печатных символов ASCII (без кириллицы)
func validDescriptionEn(s string) bool {
	if s == "" {
		return true
	}
	if len(s) < 50 || len(s) > 100 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// asyncServiceToken — постоянный ключ (8 байт), которым асинхронный сервис подписывает свои запросы
const asyncServiceToken = "bp3dTk01"

// PutTireConditionAnglesAPI — PUT /api/tire-conditions/:id/angles, body {"token": "...", "angle_x": число, "angle_y": число}
//
// @Summary     Записать новые углы поворота 3D модели услуги (результат асинхронного сервиса)
// @Description Вызывается асинхронным сервисом. Вместо JWT — постоянный ключ token в теле запроса. Углы в градусах, от 0 до 360.
// @Tags        tire-conditions
// @Accept      json
// @Produce     json
// @Param       id   path int true "ID услуги"
// @Param       body body AnglesRequest true "Ключ сервиса и новые углы"
// @Success     200 {object} repository.TireConditionResponse
// @Failure     400 "Некорректные параметры"
// @Failure     403 "Неверный ключ"
// @Failure     404 "Услуга не найдена"
// @Failure     500 "Внутренняя ошибка"
// @Router      /api/tire-conditions/{id}/angles [put]
func (h *Handler) PutTireConditionAnglesAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var body AnglesRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	if body.Token != asyncServiceToken {
		ctx.Status(http.StatusForbidden)
		return
	}
	if body.AngleX == nil || body.AngleY == nil || !validAngle(*body.AngleX) || !validAngle(*body.AngleY) {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.Repository.UpdateAngles(uint(id), *body.AngleX, *body.AngleY); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}

	tc, err := h.Repository.GetByID(id)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, tc.ToResponse())
}

// validAngle — угол в градусах от 0 до 360
func validAngle(v float64) bool {
	return v >= 0 && v <= 360
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

	if err := h.Blacklist.AddToBlacklist(ctx.Request.Context(), token, claims.UserID, time.Until(claims.ExpiresAt.Time)); err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// statusTransitionErrorCode — маппинг ошибок смены статуса услуги в HTTP-код
func statusTransitionErrorCode(err error) int {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound
	case errors.Is(err, repository.ErrNotOwner):
		return http.StatusForbidden
	case errors.Is(err, repository.ErrInvalidStatusTransition):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
