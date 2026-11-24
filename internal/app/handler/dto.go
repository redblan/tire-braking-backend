package handler

// Тела JSON-запросов API; именованные типы нужны swag для описания @Param body

type CredentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DescriptionEn — краткое описание на английском: пустое или 50–100 символов латиницей
type PublishTireConditionRequest struct {
	Description   string  `json:"description"`
	DescriptionEn string  `json:"description_en"`
	MuTire        float64 `json:"mu_tire"`
	MuSurface     float64 `json:"mu_surface"`
}

// AnglesRequest — результат асинхронного сервиса: token — постоянный ключ сервиса, углы в градусах (0–360);
// указатели — чтобы отличить отсутствующее поле от нуля
type AnglesRequest struct {
	Token  string   `json:"token"`
	AngleX *float64 `json:"angle_x"`
	AngleY *float64 `json:"angle_y"`
}

// LikeRequest — value: 1 поставить лайк, 0 снять; указатель — чтобы отличить отсутствующее поле от value=0
type LikeRequest struct {
	Value *int `json:"value"`
}
