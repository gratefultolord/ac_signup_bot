package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type Admin struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	CreatedAt time.Time `db:"created_at"`
}

type AdminRepository struct {
	db *sqlx.DB
}

func NewAdminRepository(db *sqlx.DB) *AdminRepository {
	return &AdminRepository{
		db: db,
	}
}

func (r *AdminRepository) GetAll() ([]Admin, error) {
	var admins []Admin

	err := r.db.Select(&admins, `
	    SELECT * FROM admins
	`)

	if err != nil {
		return nil, fmt.Errorf("AdminRepository.GetAll: %w", err)
	}

	return admins, nil
}

func (r *AdminRepository) Create(chatID int64) error {
	_, err := r.db.Exec(`
	    INSERT INTO admins (chat_id) VALUES ($1)
		ON CONFLICT (chat_id) DO NOTHING
	`, chatID)

	if err != nil {
		return fmt.Errorf("AdminRepository.Create: %w", err)
	}

	return nil
}

func (r *AdminRepository) CreateMessage(telegramUserID int64, firstName, lastName, message string) error {
	query := `INSERT INTO admin_messages (telegram_user_id, first_name, last_name, message) VALUES ($1, $2, $3, $4)`

	_, err := r.db.Exec(query, telegramUserID, firstName, lastName, message)
	if err != nil {
		return fmt.Errorf("AdminRepository.CreateMessage: %w", err)
	}

	return nil
}
