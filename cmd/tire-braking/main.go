package main

import "tire-braking/internal/api"

// @title                      BrakePath API
// @version                    1.0
// @description                Расчёт тормозного пути: услуги (состояния шин и покрытия), заявки, авторизация по JWT.
// @host                       localhost:8080
// @BasePath                   /
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                Введите "Bearer <access_token>"
func main() {
	api.StartServer()
}
