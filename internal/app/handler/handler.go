package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"tire-braking/internal/app/asyncclient"
	"tire-braking/internal/app/auth"
	"tire-braking/internal/app/feed"
	"tire-braking/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
	JWT        *auth.JWTManager
	Blacklist  *auth.Blacklist
	Feed       *feed.Feed
	Async      *asyncclient.Client
}

func NewHandler(r *repository.Repository, jwt *auth.JWTManager, blacklist *auth.Blacklist, f *feed.Feed, async *asyncclient.Client) *Handler {
	return &Handler{Repository: r, JWT: jwt, Blacklist: blacklist, Feed: f, Async: async}
}

// feedSessionCookie — кука с сессией гостя для ленты в Redis (не связана с авторизацией)
const feedSessionCookie = "feed_session"

// feedSession — сессия ленты в Redis: user:<id> для авторизованного пользователя,
// guest:<id из куки feed_session> для гостя; куки у гостя нет — создаётся новая
func feedSession(ctx *gin.Context) string {
	if userID, ok := currentUserID(ctx); ok {
		return "user:" + strconv.FormatUint(uint64(userID), 10)
	}
	if id, err := ctx.Cookie(feedSessionCookie); err == nil && id != "" {
		return "guest:" + id
	}

	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	id := hex.EncodeToString(buf)
	ctx.SetCookie(feedSessionCookie, id, int(feed.TTL.Seconds()), "/", "", false, true)
	return "guest:" + id
}

func parseDecimal(s string) float64 {
	s = strings.Replace(s, ",", ".", 1)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// webDefaultUserID — создатель страниц ЛР1–2, когда в браузере нет токена (как до ЛР4)
const webDefaultUserID uint = 1

// webUserID — пользователь из токена, а без токена создатель по умолчанию
func webUserID(ctx *gin.Context) uint {
	if id, ok := currentUserID(ctx); ok {
		return id
	}
	return webDefaultUserID
}

func (h *Handler) GetFeed(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	if id < 0 {
		id = 0
	}

	// БД возвращает сразу одну строку: услугу по id или следующую за ней при next=true
	current, err := h.Repository.GetFeedItem(uint(id), ctx.Query("next") == "true")
	if err != nil && id != 0 {
		// услуги с таким id нет (удалена или не опубликована) — показываем первую
		current, err = h.Repository.GetFeedItem(0, false)
	}
	if err != nil {
		logrus.Error(err)
	}

	expanded := ctx.Query("expanded") == "true"
	likes, _ := h.Repository.CountLikes(current.ID)
	current.Likes = likes
	ctx.HTML(http.StatusOK, "feed.html", gin.H{"service": current, "expanded": expanded})
}

func (h *Handler) GetAdd(ctx *gin.Context) {
	userID := webUserID(ctx)
	draft, err := h.Repository.FindDraft(userID)
	hasDraft := err == nil
	ctx.HTML(http.StatusOK, "add.html", gin.H{"service": draft, "hasDraft": hasDraft})
}

func (h *Handler) PostCreate(ctx *gin.Context) {
	name := ctx.PostForm("name")
	userID := webUserID(ctx)
	// фото и 3D модель выбираются только на шаге "Далее"; если файл не выбран, остаётся файл по умолчанию
	imageURL := uploadOptional(ctx, "photo")
	modelURL := uploadOptional(ctx, "model")
	if err := h.Repository.CreateDraft(userID, name, imageURL, modelURL); err != nil {
		logrus.Error(err)
	}
	ctx.Redirect(http.StatusFound, "/add")
}

// uploadOptional — загружает необязательный файл формы в Minio и возвращает его URL;
// пустая строка, если файл не выбран или загрузка не удалась
func uploadOptional(ctx *gin.Context, field string) string {
	file, header, err := ctx.Request.FormFile(field)
	if err != nil {
		return ""
	}
	defer file.Close()

	url, err := repository.UploadFile(file, header, "tire")
	if err != nil {
		logrus.Error(err)
		return ""
	}
	return url
}

func (h *Handler) PostPublish(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.PostForm("id"))
	name := ctx.PostForm("name")
	description := ctx.PostForm("description")
	muTire := parseDecimal(ctx.PostForm("mu_tire"))
	muSurface := parseDecimal(ctx.PostForm("mu_surface"))

	if err := h.Repository.Publish(uint(id), name, description, muTire, muSurface); err != nil {
		logrus.Error(err)
	}
	ctx.Redirect(http.StatusFound, "/tiles")
}

func (h *Handler) PostDelete(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	if err := h.Repository.Delete(uint(id)); err != nil {
		logrus.Error(err)
	}
	ctx.Redirect(http.StatusFound, "/tiles")
}

func (h *Handler) GetTiles(ctx *gin.Context) {
	gripStr := ctx.Query("grip")
	grip := parseDecimal(gripStr)
	services, err := h.Repository.SearchByMinTireGrip(grip)
	if err != nil {
		logrus.Error(err)
	}
	for i := range services {
		likes, _ := h.Repository.CountLikes(services[i].ID)
		services[i].Likes = likes
	}
	ctx.HTML(http.StatusOK, "tiles.html", gin.H{"services": services, "grip": grip})
}
