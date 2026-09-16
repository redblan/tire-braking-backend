package api

import (
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
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	r.GET("/feed/:id", h.GetFeed)
	r.GET("/add", h.GetAdd)
	r.POST("/add/create", h.PostCreate)
	r.POST("/add/publish", h.PostPublish)
	r.GET("/tiles", h.GetTiles)
	r.POST("/tiles/delete/:id", h.PostDelete)

	r.Run()
	log.Println("Server down")
}
