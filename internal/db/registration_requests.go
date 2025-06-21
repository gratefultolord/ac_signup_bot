package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AlekSi/pointer"
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

type RegistrationRequestShort struct {
	ID          int64     `db:"id"`
	FirstName   string    `db:"first_name"`
	LastName    string    `db:"last_name"`
	BirthDate   time.Time `db:"birth_date"`
	UserStatus  string    `db:"user_status"`
	PhoneNumber string    `db:"phone_number"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
	`,
		req.TelegramUserID,
		req.FirstName,
		req.LastName,
		req.BirthDate,
		req.UserStatus,
		req.DocumentPath,
		req.PhoneNumber,
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

func (r *RegistrationRequestRepository) GetNextPending() (*RegistrationRequest, error) {
	var req RegistrationRequest

	query := `
	    SELECT
		    id, telegram_user_id, first_name, last_name, birth_date, user_status, document_path,
			phone_number, status, rejection_reason, created_at, updated_at
		FROM registration_requests
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1	
	`

	err := r.db.Get(&req, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("RegistrationRequestRepository.GetNextPending: %w", err)
	}

	return &req, nil
}

func (r *RegistrationRequestRepository) GetTelegramUserIDByRequest(requestID int64) (int64, error) {
	var telegramUserID int64

	query := `
	    SELECT telegram_user_id
		FROM registration_requests
		WHERE id = $1
	`

	err := r.db.Get(&telegramUserID, query, requestID)
	if err != nil {
		return 0, fmt.Errorf("AdminRepository.GetTelegramUserIDByRequest: %w", err)
	}

	return telegramUserID, nil
}

func (r *RegistrationRequestRepository) GetByTelegramID(chatID int64) (*RegistrationRequestShort, error) {
	var req RegistrationRequestShort

	query := `
	    SELECT 
		    id, first_name, last_name, birth_date, user_status, phone_number,
			created_at, updated_at
		FROM registration_requests
		WHERE telegram_user_id = $1	
	`

	err := r.db.Get(&req, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("RegistrationRequestRepository.GetByTelegramID: %w", err)
	}

	return pointer.To(req), nil
}
