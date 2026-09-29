package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrTireConditionUnavailable = errors.New("tire condition not found or not published")
	ErrDuplicateRequestItem     = errors.New("tire condition already in this request")
	ErrInvalidStatusTransition  = errors.New("invalid status transition")
	ErrRequestNotReadyToForm    = errors.New("request is not ready to be formed")
)

// FindOrCreateDraftRequest — ORM, находит черновик заявки пользователя или создаёт новый
func (r *Repository) FindOrCreateDraftRequest(userID uint) (Request, error) {
	return findOrCreateDraftRequest(r.db, userID)
}

func findOrCreateDraftRequest(db *gorm.DB, userID uint) (Request, error) {
	var req Request
	err := db.Where("status = ? AND creator_id = ?", "draft", userID).First(&req).Error
	if err == nil {
		return req, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Request{}, err
	}

	req = Request{
		Status:    "draft",
		CreatorID: userID,
		CreatedAt: time.Now(),
	}
	err = db.Create(&req).Error
	return req, err
}

// FindDraftRequest — ORM, находит черновик заявки пользователя (без создания)
func (r *Repository) FindDraftRequest(userID uint) (Request, error) {
	var req Request
	err := r.db.Where("status = ? AND creator_id = ?", "draft", userID).First(&req).Error
	return req, err
}

// GetRequestItem — ORM, читает одну строку состава заявки по паре (request_id, tire_condition_id)
func (r *Repository) GetRequestItem(requestID, tireConditionID uint) (RequestItem, error) {
	var item RequestItem
	err := r.db.Where("request_id = ? AND tire_condition_id = ?", requestID, tireConditionID).First(&item).Error
	return item, err
}

// AddRequestItem — ORM, добавляет услугу в заявку; позиция — следующая по порядку
func (r *Repository) AddRequestItem(requestID, tireConditionID uint) (RequestItem, error) {
	return addRequestItem(r.db, requestID, tireConditionID)
}

func addRequestItem(db *gorm.DB, requestID, tireConditionID uint) (RequestItem, error) {
	var tc TireCondition
	err := db.Where("id = ? AND status = ?", tireConditionID, "published").First(&tc).Error
	if err != nil {
		return RequestItem{}, ErrTireConditionUnavailable
	}

	var existing RequestItem
	err = db.Where("request_id = ? AND tire_condition_id = ?", requestID, tireConditionID).First(&existing).Error
	if err == nil {
		return RequestItem{}, ErrDuplicateRequestItem
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return RequestItem{}, err
	}

	var count int64
	if err := db.Model(&RequestItem{}).Where("request_id = ?", requestID).Count(&count).Error; err != nil {
		return RequestItem{}, err
	}

	item := RequestItem{
		RequestID:       requestID,
		TireConditionID: tireConditionID,
		Position:        int(count),
	}
	err = db.Create(&item).Error
	return item, err
}

// AddItemToDraftRequest — ORM, находит/создаёт черновик и добавляет в него услугу одной транзакцией:
// при ошибке добавления только что созданный пустой черновик откатывается
func (r *Repository) AddItemToDraftRequest(userID, tireConditionID uint) (RequestItem, error) {
	var item RequestItem
	err := r.db.Transaction(func(tx *gorm.DB) error {
		req, err := findOrCreateDraftRequest(tx, userID)
		if err != nil {
			return err
		}

		item, err = addRequestItem(tx, req.ID, tireConditionID)
		return err
	})
	return item, err
}

// UpdateRequestItemPosition — ORM, обновляет position по паре (request_id, tire_condition_id)
func (r *Repository) UpdateRequestItemPosition(requestID, tireConditionID uint, position int) error {
	res := r.db.Model(&RequestItem{}).
		Where("request_id = ? AND tire_condition_id = ?", requestID, tireConditionID).
		Updates(map[string]interface{}{"position": position})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteRequestItem — ORM, реальное удаление строки по паре (request_id, tire_condition_id)
func (r *Repository) DeleteRequestItem(requestID, tireConditionID uint) error {
	res := r.db.Where("request_id = ? AND tire_condition_id = ?", requestID, tireConditionID).Delete(&RequestItem{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
