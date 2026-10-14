package handler

// Тела JSON-запросов API; именованные типы нужны swag для описания @Param body

type CredentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PublishTireConditionRequest struct {
	Description string  `json:"description"`
	MuTire      float64 `json:"mu_tire"`
	MuSurface   float64 `json:"mu_surface"`
}

// LikeRequest — value: 1 поставить лайк, 0 снять; указатель — чтобы отличить отсутствующее поле от value=0
type LikeRequest struct {
	Value *int `json:"value"`
}

type AddRequestItemRequest struct {
	TireConditionID uint `json:"tire_condition_id"`
}

type RequestItemPositionRequest struct {
	Position int `json:"position"`
}

type RequestSpeedRequest struct {
	SpeedKmh int `json:"speed_kmh"`
}
