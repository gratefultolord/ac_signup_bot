package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type User struct {
	ID             int64     `db:"id"`
	TelegramUserID int64     `db:"telegram_user_id"`
	FirstName      string    `db:"first_name"`
	LastName       string    `db:"last_name"`
	BirthDate      time.Time `db:"birth_date"`
	Status         string    `db:"status"`
	PhoneNumber    string    `db:"phone_number"`
	PhotoPath      *string   `db:"photo_path"`
	ExpiresAt      time.Time `db:"expires_at"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type UserRepository struct {
	db *sqlx.DB
}

func NewUsersRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

// Добавить нового пользователя (после approve)
func (r *UserRepository) Create(user *User) error {
	_, err := r.db.Exec(`
	    INSERT INTO users
		(telegram_user_id, first_name, last_name, birth_date, status, phone_number, photo_path, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		user.TelegramUserID,
		user.FirstName,
		user.LastName,
		user.BirthDate,
		user.Status,
		user.PhoneNumber,
		nil,
		user.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("UsersRepository.Create: %w", err)
	}

	return nil
}

func (r *UserRepository) GetByID(userID int64) (*User, error) {
	var user User

	err := r.db.Get(&user, `
	    SELECT * FROM users
		WHERE id = $1
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("UsersRepository.GetByID: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) GetByTelegramUserID(telegramUserID int64) (*User, error) {
	var user User

	err := r.db.Get(&user, `
	    SELECT * FROM users
		WHERE telegram_user_id = $1
	`, telegramUserID)

	if err != nil {
		return nil, fmt.Errorf("UsersRepository.GetByTelegramUserID: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) GetByPhoneNumber(phoneNumber string) (*User, error) {
	var user User

	err := r.db.Get(&user, `
	    SELECT * FROM users
		WHERE phone_number = $1
	`, phoneNumber)

	if err != nil {
		return nil, fmt.Errorf("UsersRepository.GetByPhoneNumber: %w", err)
	}

	return &user, nil
}
