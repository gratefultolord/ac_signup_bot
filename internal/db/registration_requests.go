package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type RegistrationRequest struct {
	ID              int64     `db:"id"`
	UserID          *int64    `db:"user_id"`
	TelegramUserID  int64     `db:"telegram_user_id"`
	FirstName       string    `db:"first_name"`
	LastName        string    `db:"last_name"`
	BirthDate       time.Time `db:"birth_date"`
	UserStatus      string    `db:"user_status"`
	DocumentPath    string    `db:"document_path"`
	PhoneNumber     string    `db:"phone_number"`
	Status          string    `db:"status"`
	RejectionReason *string   `db:"rejection_reason"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

type RegistrationRequestRepository struct {
	db *sqlx.DB
}

func NewRegistrationRequestRepository(db *sqlx.DB) *RegistrationRequestRepository {
	return &RegistrationRequestRepository{
		db: db,
	}
}

func (r *RegistrationRequestRepository) Create(req *RegistrationRequest) error {
	_, err := r.db.Exec(`
	    INSERT INTO registration_requests
		(telegram_user_id, first_name, last_name, birth_date, user_status,
		document_path, phone_number, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		req.TelegramUserID,
		req.FirstName,
		req.LastName,
		req.BirthDate,
		req.UserStatus,
		req.DocumentPath,
		req.PhoneNumber,
		req.Status,
	)
	if err != nil {
		return fmt.Errorf("RegistrationRequestRepository.Create: %w", err)
	}

	return nil
}

func (r *RegistrationRequestRepository) GetLatestByTelegramUserID(telegramUserID int64) (*RegistrationRequest, error) {
	var req RegistrationRequest

	err := r.db.Get(&req, `
	    SELECT * FROM registration_requests
		WHERE telegram_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, telegramUserID)

	if err != nil {
		return nil, fmt.Errorf("RegistrationRequestRepository.GetLatestByTelegramUserID: %w", err)
	}

	return &req, nil
}

func (r *RegistrationRequestRepository) UpdateDocumentAndStatus(requestID int64, telegramUserID int64, newPath string, newStatus string) error {
	_, err := r.db.Exec(`
	    UPDATE registration_requests
		SET document_path = $1, status = $2, rejection_reason = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND telegram_user_id = $4
	`, newPath, newStatus, requestID, telegramUserID)

	if err != nil {
		return fmt.Errorf("RegistrationRequestRepository.UpdateDocumentAndStatus: %w", err)
	}

	return nil
}

// Обновить статус заявки
func (r *RegistrationRequestRepository) UpdateStatus(requestID int64, newStatus string, rejectionReason *string) error {
	_, err := r.db.Exec(`
	    UPDATE registration_requests
		SET status = $1, rejection_reason = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, newStatus, rejectionReason, requestID)

	if err != nil {
		return fmt.Errorf("RegistrationRequestRepository.UpdateStatus; %w", err)
	}

	return nil
}

func (r *RegistrationRequestRepository) GetByID(requestID int64) (*RegistrationRequest, error) {
	var req RegistrationRequest

	err := r.db.Get(&req, `
	    SELECT * FROM registration_requests
		WHERE id = $1
	`, requestID)

	if err != nil {
		return nil, fmt.Errorf("RegistrationRequestRepository.GetByID: %w", err)
	}

	return &req, nil
}
