package handler

import (
	"net/http"
	"strconv"
	"strings"

	"tire-braking/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

func parseDecimal(s string) float64 {
	s = strings.Replace(s, ",", ".", 1)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func (h *Handler) GetFeed(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, _ := strconv.Atoi(idStr)

	published, err := h.Repository.GetPublished()
	if err != nil {
		logrus.Error(err)
	}
	var current repository.TireCondition
	if len(published) > 0 {
		current = published[0]
		for i, s := range published {
			if int(s.ID) == id {
				current = s
				if ctx.Query("next") == "true" {
					current = published[(i+1)%len(published)]
				}
				break
			}
		}
	}
	expanded := ctx.Query("expanded") == "true"
	likes, _ := h.Repository.CountLikes(current.ID)
	current.Likes = likes
	ctx.HTML(http.StatusOK, "feed.html", gin.H{"service": current, "expanded": expanded})
}

func (h *Handler) GetAdd(ctx *gin.Context) {
	draft, err := h.Repository.FindDraft(repository.CurrentUser())
	hasDraft := err == nil
	ctx.HTML(http.StatusOK, "add.html", gin.H{"service": draft, "hasDraft": hasDraft})
}

func (h *Handler) PostCreate(ctx *gin.Context) {
	name := ctx.PostForm("name")
	if err := h.Repository.CreateDraft(repository.CurrentUser(), name); err != nil {
		logrus.Error(err)
	}
	ctx.Redirect(http.StatusFound, "/add")
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
