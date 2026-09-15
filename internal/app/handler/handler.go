package handler

import (
	"net/http"
	"strconv"

	"tire-braking/internal/app/repository" // замени tire-braking на своё имя модуля

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

func (h *Handler) GetFeed(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, _ := strconv.Atoi(idStr)
	next := ctx.Query("next") == "true"

	service, err := h.Repository.GetService(id, next)
	if err != nil {
		logrus.Error(err)
	}
	ctx.HTML(http.StatusOK, "feed.html", gin.H{"service": service})
}

func (h *Handler) GetAdd(ctx *gin.Context) {
	draft, err := h.Repository.GetDraft()
	if err != nil {
		logrus.Error(err)
	}
	ctx.HTML(http.StatusOK, "add.html", gin.H{"service": draft})
}

func (h *Handler) GetTiles(ctx *gin.Context) {
	gripStr := ctx.Query("grip")
	minGrip, _ := strconv.ParseFloat(gripStr, 64) // если пусто — 0, вернёт всё

	services, err := h.Repository.GetServicesByMinTireGrip(minGrip)
	if err != nil {
		logrus.Error(err)
	}
	ctx.HTML(http.StatusOK, "tiles.html", gin.H{"services": services, "grip": gripStr})
}
