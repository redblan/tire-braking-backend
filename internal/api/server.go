package api

import (
	"html/template"
	"log"

	_ "tire-braking/docs"
	"tire-braking/internal/app/auth"
	"tire-braking/internal/app/config"
	"tire-braking/internal/app/handler"
	"tire-braking/internal/app/redis"
	"tire-braking/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func StartServer() {
	log.Println("Starting server")

	repo, err := repository.NewRepository()
	if err != nil {
		logrus.Error("ошибка инициализации репозитория")
	}

	cfg := config.Load()
	rdb, err := redis.New(cfg)
	if err != nil {
		logrus.Fatalf("ошибка подключения к Redis: %v", err)
	}
	h := handler.NewHandler(repo, auth.NewJWTManager(cfg), auth.NewBlacklist(rdb))

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

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// HTML-страницы ЛР1: просмотр — гость, изменение — авторизованный
	r.GET("/feed/:id", h.GetFeed)
	r.GET("/tiles", h.GetTiles)

	web := r.Group("", h.OptionalAuth, h.RequireAuth)
	web.GET("/add", h.GetAdd)
	web.POST("/add/create", h.PostCreate)
	web.POST("/add/publish", h.PostPublish)
	web.POST("/tiles/delete/:id", h.PostDelete)

	// гость: регистрация, вход, чтение услуг (is_owner = false)
	api := r.Group("/api", h.OptionalAuth)
	api.POST("/register", h.PostRegisterAPI)
	api.POST("/auth", h.PostAuthAPI)
	api.GET("/tire-conditions", h.GetTireConditionsAPI)
	api.GET("/tire-conditions/feed", h.GetTireConditionsFeedAPI)
	api.GET("/tire-conditions/:id", h.GetTireConditionByIDAPI)

	// авторизованный пользователь
	user := api.Group("", h.RequireAuth)
	user.POST("/logout", h.PostLogoutAPI)

	user.GET("/tire-conditions/draft", h.GetMyDraftTireConditionAPI)
	user.POST("/tire-conditions", h.PostTireConditionAPI)
	user.PUT("/tire-conditions/:id/publish", h.PutPublishTireConditionAPI)
	user.DELETE("/tire-conditions/:id", h.DeleteTireConditionAPI)
	user.POST("/tire-conditions/:id/like", h.PostLikeTireConditionAPI)

	user.POST("/requests/items", h.PostRequestItemAPI)
	user.PUT("/requests/items/:tire_condition_id", h.PutRequestItemAPI)
	user.DELETE("/requests/items/:tire_condition_id", h.DeleteRequestItemAPI)

	user.GET("/requests/cart", h.GetCartAPI)
	user.GET("/requests", h.GetRequestsListAPI)
	user.GET("/requests/:id", h.GetRequestByIDAPI)
	user.PUT("/requests/:id", h.PutRequestSpeedAPI)
	user.PUT("/requests/:id/form", h.PutRequestFormAPI)
	user.DELETE("/requests/:id", h.DeleteRequestAPI)

	// модератор
	moderator := api.Group("", h.RequireModerator)
	moderator.PUT("/requests/:id/complete", h.PutRequestCompleteAPI)
	moderator.PUT("/requests/:id/reject", h.PutRequestRejectAPI)

	r.Run()
	log.Println("Server down")
}
