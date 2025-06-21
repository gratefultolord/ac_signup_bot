package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type Token struct {
	ID          int64      `db:"id"`
	UserID      int64      `db:"user_id"`
	Token       *string    `db:"token"`
	Code        string     `db:"code"`
	PhoneNumber string     `db:"phone_number"`
	CreatedAt   time.Time  `db:"created_at"`
	ExpiresAt   *time.Time `db:"expires_at"`
}

type TokenRepository struct {
	db *sqlx.DB
}

func NewTokenRepository(db *sqlx.DB) *TokenRepository {
	return &TokenRepository{
		db: db,
	}
}

// Создать новый токен
func (r *TokenRepository) Create(token *Token) error {
	_, err := r.db.Exec(`
        INSERT INTO tokens
        (user_id, token, code, phone_number)
        VALUES ($1, $2, $3, $4)
    `,
		token.UserID,
		token.Token,
		token.Code,
		token.PhoneNumber,
	)
	if err != nil {
		return fmt.Errorf("TokenRepository.Create: %w", err)
	}
	return nil
}

// Получить токен по одноразовому коду
func (r *TokenRepository) GetByCode(code string) (*Token, error) {
	var token Token
	err := r.db.Get(&token, `
        SELECT * FROM tokens
        WHERE code = $1
    `, code)
	if err != nil {
		return nil, fmt.Errorf("TokenRepository.GetByCode: %w", err)
	}
	return &token, nil
}

// Обновить JWT токен по id (после верификации кода на сайте)
func (r *TokenRepository) UpdateJWT(tokenID int64, jwtToken string) error {
	_, err := r.db.Exec(`
        UPDATE tokens
        SET token = $1
        WHERE id = $2
    `, jwtToken, tokenID)
	if err != nil {
		return fmt.Errorf("TokenRepository.UpdateJWT: %w", err)
	}
	return nil
}

// Удалить токен по JWT
func (r *TokenRepository) DeleteByToken(jwtToken string) error {
	_, err := r.db.Exec(`
        DELETE FROM tokens
        WHERE token = $1
    `, jwtToken)
	if err != nil {
		return fmt.Errorf("TokenRepository.DeleteByToken: %w", err)
	}
	return nil
}

// Очистить устаревшие токены
func (r *TokenRepository) DeleteExpiredTokens() error {
	_, err := r.db.Exec(`
        DELETE FROM tokens
        WHERE expires_at < NOW()
    `)
	if err != nil {
		return fmt.Errorf("TokenRepository.DeleteExpiredTokens: %w", err)
	}
	return nil
}
