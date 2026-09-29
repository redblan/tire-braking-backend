package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// RequestListRow — промежуточная строка списка заявок (join с users, агрегат по request_items)
type RequestListRow struct {
	ID                uint
	Status            string
	CreatorID         uint
	ModeratorID       *uint
	SpeedKmh          *int
	CreatedAt         time.Time
	FormedAt          *time.Time
	CompletedAt       *time.Time
	CreatorUsername   string
	ModeratorUsername *string
	ResultCount       int64
}

// RequestItemDetail — строка состава заявки (join request_items + tire_conditions)
type RequestItemDetail struct {
	RequestItemID   uint
	TireConditionID uint
	Name            string
	ImageURL        string
	Position        int
	BrakingDistance *float64
}

// GetCartInfo — ORM, инфо о текущей корзине (draft-заявке) пользователя; отсутствие черновика — не ошибка
func (r *Repository) GetCartInfo(userID uint) (*uint, int, error) {
	var req Request
	err := r.db.Where("status = ? AND creator_id = ?", "draft", userID).First(&req).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	var count int64
	if err := r.db.Model(&RequestItem{}).Where("request_id = ?", req.ID).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	id := req.ID
	return &id, int(count), nil
}

// GetRequestsList — ORM, список заявок (без draft/deleted) с именами создателя/модератора и числом посчитанных позиций
func (r *Repository) GetRequestsList(statusFilter string, dateFrom, dateTo *time.Time) ([]RequestListRow, error) {
	var rows []RequestListRow

	query := r.db.Table("requests AS r").
		Select(`r.id, r.status, r.creator_id, r.moderator_id, r.speed_kmh, r.created_at, r.formed_at, r.completed_at,
			creator.username AS creator_username, moderator.username AS moderator_username,
			(SELECT COUNT(*) FROM request_items ri WHERE ri.request_id = r.id AND ri.braking_distance IS NOT NULL) AS result_count`).
		Joins("JOIN users AS creator ON creator.id = r.creator_id").
		Joins("LEFT JOIN users AS moderator ON moderator.id = r.moderator_id").
		Where("r.status NOT IN ?", []string{"draft", "deleted"})

	if statusFilter != "" {
		query = query.Where("r.status = ?", statusFilter)
	}
	if dateFrom != nil {
		query = query.Where("r.formed_at >= ?", *dateFrom)
	}
	if dateTo != nil {
		query = query.Where("r.formed_at <= ?", *dateTo)
	}

	err := query.Order("r.id").Scan(&rows).Error
	return rows, err
}

// GetRequestWithItems — ORM, заявка + её состав (join с tire_conditions); заявка не найдена -> gorm.ErrRecordNotFound
func (r *Repository) GetRequestWithItems(id uint) (Request, []RequestItemDetail, error) {
	var req Request
	if err := r.db.First(&req, id).Error; err != nil {
		return Request{}, nil, err
	}

	var items []RequestItemDetail
	err := r.db.Table("request_items AS ri").
		Select("ri.id AS request_item_id, ri.tire_condition_id, tc.name, tc.image_url, ri.position, ri.braking_distance").
		Joins("JOIN tire_conditions AS tc ON tc.id = ri.tire_condition_id").
		Where("ri.request_id = ?", id).
		Order("ri.position").
		Scan(&items).Error

	return req, items, err
}

// UpdateRequestSpeed — ORM, обновляет speed_kmh; заявка не найдена -> gorm.ErrRecordNotFound
func (r *Repository) UpdateRequestSpeed(id uint, speedKmh int) error {
	res := r.db.Model(&Request{}).Where("id = ?", id).Update("speed_kmh", speedKmh)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FormRequest — в транзакции: draft -> formed, считает braking_distance для каждой позиции по формуле S = v²/(2·mu_tire·mu_surface·9.81)
func (r *Repository) FormRequest(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var req Request
		if err := tx.First(&req, id).Error; err != nil {
			return err
		}
		if req.Status != "draft" {
			return ErrInvalidStatusTransition
		}

		var items []RequestItem
		if err := tx.Where("request_id = ?", id).Find(&items).Error; err != nil {
			return err
		}
		if req.SpeedKmh == nil || len(items) == 0 {
			return ErrRequestNotReadyToForm
		}

		speedMs := float64(*req.SpeedKmh) / 3.6

		for _, item := range items {
			var tc TireCondition
			if err := tx.First(&tc, item.TireConditionID).Error; err != nil {
				return err
			}

			distance := (speedMs * speedMs) / (2 * tc.MuTire * tc.MuSurface * 9.81)
			if err := tx.Model(&RequestItem{}).Where("id = ?", item.ID).Update("braking_distance", distance).Error; err != nil {
				return err
			}
		}

		return tx.Model(&Request{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":    "formed",
			"formed_at": time.Now(),
		}).Error
	})
}

func transitionFormedTo(tx *gorm.DB, id, moderatorID uint, newStatus string) error {
	var req Request
	if err := tx.First(&req, id).Error; err != nil {
		return err
	}
	if req.Status != "formed" {
		return ErrInvalidStatusTransition
	}

	return tx.Model(&Request{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       newStatus,
		"moderator_id": moderatorID,
		"completed_at": time.Now(),
	}).Error
}

// CompleteRequest — formed -> completed (только модератор, сейчас — тот же CurrentUser())
func (r *Repository) CompleteRequest(id, moderatorID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return transitionFormedTo(tx, id, moderatorID, "completed")
	})
}

// RejectRequest — formed -> rejected (только модератор, сейчас — тот же CurrentUser())
func (r *Repository) RejectRequest(id, moderatorID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return transitionFormedTo(tx, id, moderatorID, "rejected")
	})
}

// DeleteRequest — draft -> deleted, логическое удаление
func (r *Repository) DeleteRequest(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var req Request
		if err := tx.First(&req, id).Error; err != nil {
			return err
		}
		if req.Status != "draft" {
			return ErrInvalidStatusTransition
		}

		return tx.Model(&Request{}).Where("id = ?", id).Update("status", "deleted").Error
	})
}
