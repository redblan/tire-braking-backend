package api

import (
	"html/template"
	"log"

	_ "tire-braking/docs"
	"tire-braking/internal/app/asyncclient"
	"tire-braking/internal/app/auth"
	"tire-braking/internal/app/config"
	"tire-braking/internal/app/feed"
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
	h := handler.NewHandler(repo, auth.NewJWTManager(cfg), auth.NewBlacklist(rdb), feed.New(rdb, repo), asyncclient.New(cfg.AsyncServiceURL))

	r := gin.Default()
	r.Use(handler.CORS)
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

	// HTML-страницы ЛР1–2 открываются в браузере без токена; если токен передан — действия от его имени
	r.GET("/feed/:id", h.GetFeed)
	r.GET("/tiles", h.GetTiles)

	web := r.Group("", h.OptionalAuth)
	web.GET("/add", h.GetAdd)
	web.POST("/add/create", h.PostCreate)
	web.POST("/add/publish", h.PostPublish)
	web.POST("/tiles/delete/:id", h.PostDelete)

	// гость: регистрация, вход, чтение услуг (is_owner = 0)
	api := r.Group("/api", h.OptionalAuth)
	api.POST("/register", h.PostRegisterAPI)
	api.POST("/auth", h.PostAuthAPI)
	api.GET("/tire-conditions", h.GetTireConditionsAPI)
	api.GET("/tire-conditions/feed", h.GetTireConditionsFeedAPI)
	// асинхронный сервис: вместо JWT — постоянный ключ в теле запроса
	api.PUT("/tire-conditions/:id/angles", h.PutTireConditionAnglesAPI)

	// авторизованный пользователь
	user := api.Group("", h.RequireAuth)
	user.POST("/logout", h.PostLogoutAPI)

	user.GET("/tire-conditions/draft", h.GetMyDraftTireConditionAPI)
	user.POST("/tire-conditions", h.PostTireConditionAPI)
	user.PUT("/tire-conditions/:id/publish", h.PutPublishTireConditionAPI)
	user.DELETE("/tire-conditions/:id", h.DeleteTireConditionAPI)
	user.POST("/tire-conditions/:id/like", h.PostLikeTireConditionAPI)

	r.Run()
	log.Println("Server down")
}
