package repository

import (
	"errors"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type TireCondition struct {
	ID          uint       `gorm:"primaryKey"`
	Name        string     `gorm:"column:name"`
	Description string     `gorm:"column:description"`
	Status      string     `gorm:"column:status"`
	ImageURL    string     `gorm:"column:image_url"`
	VideoURL    string     `gorm:"column:video_url"`
	MuTire      float64    `gorm:"column:mu_tire"`
	MuSurface   float64    `gorm:"column:mu_surface"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	CreatorID   uint       `gorm:"column:creator_id"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	Likes       int64      `gorm:"-"`
}

func (TireCondition) TableName() string { return "tire_conditions" }

var ErrNotOwner = errors.New("tire condition belongs to another user")

type Repository struct {
	db *gorm.DB
}

func NewRepository() (*Repository, error) {
	dsn := "host=localhost user=postgres password=postgres dbname=brakepath port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// GetPublished — ORM, для ленты и как база для плитки
func (r *Repository) GetPublished() ([]TireCondition, error) {
	var res []TireCondition
	err := r.db.Where("status = ?", "published").Order("id").Find(&res).Error
	return res, err
}

// SearchByMinTireGrip — ORM, поиск/фильтр для плитки
func (r *Repository) SearchByMinTireGrip(min float64) ([]TireCondition, error) {
	var res []TireCondition
	err := r.db.Where("status = ? AND mu_tire >= ?", "published", min).Order("id").Find(&res).Error
	return res, err
}

func (r *Repository) GetByID(id int) (TireCondition, error) {
	var tc TireCondition
	err := r.db.First(&tc, id).Error
	return tc, err
}

// FindDraft — есть ли уже черновик у этого пользователя
func (r *Repository) FindDraft(creatorID uint) (TireCondition, error) {
	var tc TireCondition
	err := r.db.Where("status = ? AND creator_id = ?", "draft", creatorID).First(&tc).Error
	return tc, err
}

// FindMyDraftTireCondition — ORM, черновик услуги пользователя для GET /api/tire-conditions/draft
func (r *Repository) FindMyDraftTireCondition(userID uint) (TireCondition, error) {
	var tc TireCondition
	err := r.db.Where("status = ? AND creator_id = ?", "draft", userID).First(&tc).Error
	return tc, err
}

// CreateDraft — ORM, создаёт пустой черновик (кнопка "Далее" в первый раз)
func (r *Repository) CreateDraft(creatorID uint, name string) error {
	tc := TireCondition{
		Name:      name,
		Status:    "draft",
		ImageURL:  "/static/images/default_tire.jpg",
		VideoURL:  "/static/videos/default_tire.mp4",
		CreatorID: creatorID,
		CreatedAt: time.Now(),
	}
	return r.db.Create(&tc).Error
}

// CreateTireCondition — ORM, создаёт услугу через API (POST /api/tire-conditions), статус "draft" по умолчанию
func (r *Repository) CreateTireCondition(name, description, imageURL, videoURL string, muTire, muSurface float64, creatorID uint) (TireCondition, error) {
	tc := TireCondition{
		Name:        name,
		Description: description,
		Status:      "draft",
		ImageURL:    imageURL,
		VideoURL:    videoURL,
		MuTire:      muTire,
		MuSurface:   muSurface,
		CreatorID:   creatorID,
		CreatedAt:   time.Now(),
	}
	err := r.db.Create(&tc).Error
	return tc, err
}

// Publish — ORM, смена статуса черновика на опубликован + заполнение полей темы
func (r *Repository) Publish(id uint, name, description string, muTire, muSurface float64) error {
	now := time.Now()
	return r.db.Model(&TireCondition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":         name,
		"description":  description,
		"status":       "published",
		"mu_tire":      muTire,
		"mu_surface":   muSurface,
		"published_at": now,
	}).Error
}

// PublishTireCondition — ORM, в транзакции: draft -> published + заполнение полей; не черновик -> ErrInvalidStatusTransition
func (r *Repository) PublishTireCondition(id uint, description string, muTire, muSurface float64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tc TireCondition
		if err := tx.First(&tc, id).Error; err != nil {
			return err
		}
		if tc.Status != "draft" {
			return ErrInvalidStatusTransition
		}

		return tx.Model(&TireCondition{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":       "published",
			"description":  description,
			"mu_tire":      muTire,
			"mu_surface":   muSurface,
			"published_at": time.Now(),
		}).Error
	})
}

// Delete — ЧЕРЕЗ СЫРОЙ SQL UPDATE, без ORM (специально по методичке)
func (r *Repository) Delete(id uint) error {
	return r.db.Exec("UPDATE tire_conditions SET status = 'deleted' WHERE id = ?", id).Error
}

// DeleteTireCondition — владелец проверяется через ORM, само удаление — сырой SQL UPDATE (логическое удаление)
func (r *Repository) DeleteTireCondition(id, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tc TireCondition
		if err := tx.First(&tc, id).Error; err != nil {
			return err
		}
		if tc.CreatorID != userID {
			return ErrNotOwner
		}

		return tx.Exec("UPDATE tire_conditions SET status = 'deleted' WHERE id = ?", id).Error
	})
}

// ToggleLike — ORM: value=1 ставит лайк (без дублей по паре user_id+tire_condition_id), value=0 снимает;
// услуга не найдена -> gorm.ErrRecordNotFound
func (r *Repository) ToggleLike(tireConditionID, userID uint, value int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tc TireCondition
		if err := tx.First(&tc, tireConditionID).Error; err != nil {
			return err
		}

		if value == 0 {
			return tx.Where("user_id = ? AND tire_condition_id = ?", userID, tireConditionID).Delete(&Like{}).Error
		}

		var existing Like
		err := tx.Where("user_id = ? AND tire_condition_id = ?", userID, tireConditionID).First(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		return tx.Create(&Like{UserID: userID, TireConditionID: tireConditionID}).Error
	})
}

func (r *Repository) CountLikes(tireConditionID uint) (int64, error) {
	var count int64
	err := r.db.Table("likes").Where("tire_condition_id = ?", tireConditionID).Count(&count).Error
	return count, err
}
