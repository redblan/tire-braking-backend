package api

import (
	"html/template"
	"log"

	"tire-braking/internal/app/handler"
	"tire-braking/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func StartServer() {
	log.Println("Starting server")

	repo, err := repository.NewRepository()
	if err != nil {
		logrus.Error("ошибка инициализации репозитория")
	}
	h := handler.NewHandler(repo)

	r := gin.Default()
	r.SetFuncMap(template.FuncMap{
		"truncate": func(s string, n int) string {
			r := []rune(s)
			if len(r) > n {
				return string(r[:n]) + "..."
			}
			return s
		},
	})
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	r.GET("/feed/:id", h.GetFeed)
	r.GET("/add", h.GetAdd)
	r.POST("/add/create", h.PostCreate)
	r.POST("/add/publish", h.PostPublish)
	r.GET("/tiles", h.GetTiles)
	r.POST("/tiles/delete/:id", h.PostDelete)

	r.GET("/api/tire-conditions", h.GetTireConditionsAPI)
	r.GET("/api/tire-conditions/feed", h.GetTireConditionsFeedAPI)
	r.GET("/api/tire-conditions/draft", h.GetMyDraftTireConditionAPI)
	r.GET("/api/tire-conditions/:id", h.GetTireConditionByIDAPI)
	r.POST("/api/tire-conditions", h.PostTireConditionAPI)
	r.PUT("/api/tire-conditions/:id/publish", h.PutPublishTireConditionAPI)
	r.DELETE("/api/tire-conditions/:id", h.DeleteTireConditionAPI)
	r.POST("/api/tire-conditions/:id/like", h.PostLikeTireConditionAPI)

	r.POST("/api/requests/items", h.PostRequestItemAPI)
	r.PUT("/api/requests/items/:tire_condition_id", h.PutRequestItemAPI)
	r.DELETE("/api/requests/items/:tire_condition_id", h.DeleteRequestItemAPI)

	r.GET("/api/requests/cart", h.GetCartAPI)
	r.GET("/api/requests", h.GetRequestsListAPI)
	r.GET("/api/requests/:id", h.GetRequestByIDAPI)
	r.PUT("/api/requests/:id", h.PutRequestSpeedAPI)
	r.PUT("/api/requests/:id/form", h.PutRequestFormAPI)
	r.PUT("/api/requests/:id/complete", h.PutRequestCompleteAPI)
	r.PUT("/api/requests/:id/reject", h.PutRequestRejectAPI)
	r.DELETE("/api/requests/:id", h.DeleteRequestAPI)

	r.POST("/api/register", h.PostRegisterAPI)
	r.POST("/api/auth", h.PostAuthAPI)
	r.POST("/api/logout", h.PostLogoutAPI)

	r.Run()
	log.Println("Server down")
}
