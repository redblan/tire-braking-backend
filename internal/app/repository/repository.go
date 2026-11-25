package repository

import (
	"errors"
	"math"
	"math/rand"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type TireCondition struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"column:name"`
	Description string `gorm:"column:description"`
	// DescriptionEn — краткое описание на английском (50–100 символов), по нему считается эмбеддинг для поиска по изображению
	DescriptionEn string     `gorm:"column:description_en"`
	Status        string     `gorm:"column:status"`
	ImageURL      string     `gorm:"column:image_url"`
	ModelURL      string     `gorm:"column:model_url"`
	MuTire        float64    `gorm:"column:mu_tire"`
	MuSurface     float64    `gorm:"column:mu_surface"`
	AngleX        float64    `gorm:"column:angle_x"`
	AngleY        float64    `gorm:"column:angle_y"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	CreatorID     uint       `gorm:"column:creator_id"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	Likes         int64      `gorm:"-"`
}

func (TireCondition) TableName() string { return "tire_conditions" }

var (
	ErrNotOwner                = errors.New("object belongs to another user")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)

// файлы по умолчанию: подставляются, если при создании услуги фото или 3D модель не выбраны
const (
	DefaultImageURL = "/static/images/default_tire.jpg"
	DefaultModelURL = "/static/models/default_tire.glb"
)

type Repository struct {
	db *gorm.DB
}

// RandomAngle — случайный угол поворота 3D модели в градусах, 0–359.99 (два знака, как в NUMERIC(5,2))
func RandomAngle() float64 {
	return math.Floor(rand.Float64()*36000) / 100
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

// GetPublishedPage — ORM, одна страница опубликованных услуг для плитки и общее количество подходящих записей (total);
// hasGrip = true — только услуги с mu_tire >= minGrip
func (r *Repository) GetPublishedPage(minGrip float64, hasGrip bool, limit, offset int) ([]TireCondition, int64, error) {
	base := r.db.Model(&TireCondition{}).Where("status = ?", "published")
	if hasGrip {
		base = base.Where("mu_tire >= ?", minGrip)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var res []TireCondition
	err := base.Session(&gorm.Session{}).Order("id").Limit(limit).Offset(offset).Find(&res).Error
	return res, total, err
}

// RandomPublishedIDs — ORM, id случайных неповторяющихся опубликованных услуг (для ленты в Redis)
func (r *Repository) RandomPublishedIDs(limit int) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&TireCondition{}).Where("status = ?", "published").Order("random()").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

// GetFeedItem — ORM, одна опубликованная услуга для ленты:
// id = 0 — первая; next = false — услуга с этим id; next = true — следующая после id (после последней — первая)
func (r *Repository) GetFeedItem(id uint, next bool) (TireCondition, error) {
	var tc TireCondition
	base := r.db.Where("status = ?", "published")

	switch {
	case id == 0:
		err := base.Order("id").First(&tc).Error
		return tc, err
	case !next:
		err := base.Where("id = ?", id).First(&tc).Error
		return tc, err
	}

	err := r.db.Where("status = ? AND id > ?", "published", id).Order("id").First(&tc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = r.db.Where("status = ?", "published").Order("id").First(&tc).Error
	}
	return tc, err
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

// CreateDraft — ORM, создаёт черновик (кнопка "Далее" в первый раз); если фото или 3D модель не выбраны — файлы по умолчанию
func (r *Repository) CreateDraft(creatorID uint, name, imageURL, modelURL string) error {
	if imageURL == "" {
		imageURL = DefaultImageURL
	}
	if modelURL == "" {
		modelURL = DefaultModelURL
	}
	tc := TireCondition{
		Name:      name,
		Status:    "draft",
		ImageURL:  imageURL,
		ModelURL:  modelURL,
		AngleX:    RandomAngle(),
		AngleY:    RandomAngle(),
		CreatorID: creatorID,
		CreatedAt: time.Now(),
	}
	return r.db.Create(&tc).Error
}

// CreateTireCondition — ORM, создаёт услугу через API (POST /api/tire-conditions), статус "draft" по умолчанию;
// на шаге «Далее» приходят только название, изображение и 3D модель — описание и коэффициенты сцепления
// заполняются при публикации; углы поворота 3D модели инициализируются случайно
func (r *Repository) CreateTireCondition(name, imageURL, modelURL string, creatorID uint) (TireCondition, error) {
	tc := TireCondition{
		Name:      name,
		Status:    "draft",
		ImageURL:  imageURL,
		ModelURL:  modelURL,
		AngleX:    RandomAngle(),
		AngleY:    RandomAngle(),
		CreatorID: creatorID,
		CreatedAt: time.Now(),
	}
	err := r.db.Create(&tc).Error
	return tc, err
}

// UpdateAngles — ORM, записывает новые углы поворота 3D модели услуги (результат асинхронного сервиса);
// услуга не найдена -> gorm.ErrRecordNotFound
func (r *Repository) UpdateAngles(id uint, angleX, angleY float64) error {
	res := r.db.Model(&TireCondition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"angle_x": angleX,
		"angle_y": angleY,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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

// PublishTireCondition — ORM, в транзакции: draft -> published + заполнение полей; не черновик -> ErrInvalidStatusTransition;
// чужая услуга -> ErrNotOwner (модератору проверка владельца не нужна)
func (r *Repository) PublishTireCondition(id uint, description, descriptionEn string, muTire, muSurface float64, userID uint, isModerator bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tc TireCondition
		if err := tx.First(&tc, id).Error; err != nil {
			return err
		}
		if !isModerator && tc.CreatorID != userID {
			return ErrNotOwner
		}
		if tc.Status != "draft" {
			return ErrInvalidStatusTransition
		}

		return tx.Model(&TireCondition{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":         "published",
			"description":    description,
			"description_en": descriptionEn,
			"mu_tire":        muTire,
			"mu_surface":     muSurface,
			"published_at":   time.Now(),
		}).Error
	})
}

// Delete — ЧЕРЕЗ СЫРОЙ SQL UPDATE, без ORM (специально по методичке)
func (r *Repository) Delete(id uint) error {
	return r.db.Exec("UPDATE tire_conditions SET status = 'deleted' WHERE id = ?", id).Error
}

// DeleteTireCondition — владелец проверяется через ORM (модератору — не нужно), само удаление — сырой SQL UPDATE (логическое удаление)
func (r *Repository) DeleteTireCondition(id, userID uint, isModerator bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var tc TireCondition
		if err := tx.First(&tc, id).Error; err != nil {
			return err
		}
		if !isModerator && tc.CreatorID != userID {
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

// IsLiked — стоит ли лайк пользователя на услуге
func (r *Repository) IsLiked(tireConditionID, userID uint) (bool, error) {
	var count int64
	err := r.db.Table("likes").Where("tire_condition_id = ? AND user_id = ?", tireConditionID, userID).Count(&count).Error
	return count > 0, err
}
