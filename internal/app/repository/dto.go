package repository

// TireConditionResponse — DTO для JSON-ответов по Услуге.
// Статус (внутреннее поле БД) наружу не отдаётся.
// IsOwner — признак 0/1: 1, если создатель услуги совпадает с текущим пользователем (для гостя всегда 0).
// Заполняется в хендлере — там известен текущий пользователь.
// AngleX, AngleY — углы поворота 3D модели в градусах (0–360).
type TireConditionResponse struct {
	ID            uint    `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	DescriptionEn string  `json:"description_en"`
	ImageURL      string  `json:"image_url"`
	ModelURL      string  `json:"model_url"`
	MuTire        float64 `json:"mu_tire"`
	MuSurface     float64 `json:"mu_surface"`
	AngleX        float64 `json:"angle_x"`
	AngleY        float64 `json:"angle_y"`
	IsOwner       int     `json:"is_owner"`
}

// TireConditionsPage — DTO для GET /api/tire-conditions: страница списка услуг и общее количество записей
type TireConditionsPage struct {
	Total int64                   `json:"total"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
	Data  []TireConditionResponse `json:"data"`
}

// FeedResponse — DTO для GET /api/tire-conditions/feed: одна услуга ленты.
// IsLiked — признак 0/1: стоит ли лайк текущего пользователя на этой услуге (для гостя всегда 0).
// Position — порядковый номер услуги в ленте текущей сессии (1..100); 0, если услуги нет в ленте сессии.
type FeedResponse struct {
	TireConditionResponse
	IsLiked  int `json:"is_liked"`
	Position int `json:"position"`
}

func (tc TireCondition) ToResponse() TireConditionResponse {
	return TireConditionResponse{
		ID:            tc.ID,
		Name:          tc.Name,
		Description:   tc.Description,
		DescriptionEn: tc.DescriptionEn,
		ImageURL:      tc.ImageURL,
		ModelURL:      tc.ModelURL,
		MuTire:        tc.MuTire,
		MuSurface:     tc.MuSurface,
		AngleX:        tc.AngleX,
		AngleY:        tc.AngleY,
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
