package repository

import "time"

// TireConditionResponse — DTO для JSON-ответов по Услуге.
// Статус (внутреннее поле БД) наружу не отдаётся.
// IsOwner заполняется в хендлере — там известен текущий пользователь.
type TireConditionResponse struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	VideoURL    string  `json:"video_url"`
	MuTire      float64 `json:"mu_tire"`
	MuSurface   float64 `json:"mu_surface"`
	IsOwner     bool    `json:"is_owner"`
}

func (tc TireCondition) ToResponse() TireConditionResponse {
	return TireConditionResponse{
		ID:          tc.ID,
		Name:        tc.Name,
		Description: tc.Description,
		ImageURL:    tc.ImageURL,
		VideoURL:    tc.VideoURL,
		MuTire:      tc.MuTire,
		MuSurface:   tc.MuSurface,
	}
}

// RequestItemResponse — DTO для JSON-ответов по составу заявки (м-м)
type RequestItemResponse struct {
	ID              uint     `json:"id"`
	RequestID       uint     `json:"request_id"`
	TireConditionID uint     `json:"tire_condition_id"`
	Position        int      `json:"position"`
	BrakingDistance *float64 `json:"braking_distance"`
}

func (ri RequestItem) ToResponse() RequestItemResponse {
	return RequestItemResponse{
		ID:              ri.ID,
		RequestID:       ri.RequestID,
		TireConditionID: ri.TireConditionID,
		Position:        ri.Position,
		BrakingDistance: ri.BrakingDistance,
	}
}

// UserResponse — DTO для POST /api/register; хеш пароля наружу не отдаётся
type UserResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

func (u User) ToResponse() UserResponse {
	return UserResponse{ID: u.ID, Username: u.Username}
}

// CartResponse — DTO для GET /api/requests/cart
type CartResponse struct {
	RequestID  *uint `json:"request_id"`
	ItemsCount int   `json:"items_count"`
}

// RequestListItemResponse — DTO для строки списка заявок (GET /api/requests)
type RequestListItemResponse struct {
	ID                uint       `json:"id"`
	Status            string     `json:"status"`
	CreatorUsername   string     `json:"creator_username"`
	ModeratorUsername *string    `json:"moderator_username"`
	SpeedKmh          *int       `json:"speed_kmh"`
	CreatedAt         time.Time  `json:"created_at"`
	FormedAt          *time.Time `json:"formed_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	ResultCount       int64      `json:"result_count"`
}

func (row RequestListRow) ToResponse() RequestListItemResponse {
	return RequestListItemResponse{
		ID:                row.ID,
		Status:            row.Status,
		CreatorUsername:   row.CreatorUsername,
		ModeratorUsername: row.ModeratorUsername,
		SpeedKmh:          row.SpeedKmh,
		CreatedAt:         row.CreatedAt,
		FormedAt:          row.FormedAt,
		CompletedAt:       row.CompletedAt,
		ResultCount:       row.ResultCount,
	}
}

// RequestItemDetailResponse — DTO для позиции состава заявки внутри RequestDetailResponse
type RequestItemDetailResponse struct {
	ID              uint     `json:"id"`
	TireConditionID uint     `json:"tire_condition_id"`
	Name            string   `json:"name"`
	ImageURL        string   `json:"image_url"`
	Position        int      `json:"position"`
	BrakingDistance *float64 `json:"braking_distance"`
}

func (d RequestItemDetail) ToResponse() RequestItemDetailResponse {
	return RequestItemDetailResponse{
		ID:              d.RequestItemID,
		TireConditionID: d.TireConditionID,
		Name:            d.Name,
		ImageURL:        d.ImageURL,
		Position:        d.Position,
		BrakingDistance: d.BrakingDistance,
	}
}

// RequestDetailResponse — DTO для GET /api/requests/:id и успешных PUT-переходов статуса
type RequestDetailResponse struct {
	ID          uint                        `json:"id"`
	Status      string                      `json:"status"`
	SpeedKmh    *int                        `json:"speed_kmh"`
	CreatedAt   time.Time                   `json:"created_at"`
	FormedAt    *time.Time                  `json:"formed_at"`
	CompletedAt *time.Time                  `json:"completed_at"`
	Items       []RequestItemDetailResponse `json:"items"`
}

func ToRequestDetailResponse(req Request, items []RequestItemDetail) RequestDetailResponse {
	itemResponses := make([]RequestItemDetailResponse, len(items))
	for i, item := range items {
		itemResponses[i] = item.ToResponse()
	}

	return RequestDetailResponse{
		ID:          req.ID,
		Status:      req.Status,
		SpeedKmh:    req.SpeedKmh,
		CreatedAt:   req.CreatedAt,
		FormedAt:    req.FormedAt,
		CompletedAt: req.CompletedAt,
		Items:       itemResponses,
	}
}
