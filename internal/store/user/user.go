package user

import (
	"agenda-automator-api/internal/domain"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserStore handles user-related database operations
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore creates a new UserStore
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// CreateUser maakt een nieuwe gebruiker aan in de database
func (s *UserStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	query := `
    INSERT INTO users (email, name)
    VALUES ($1, $2)
    ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
    RETURNING id, email, name, created_at, updated_at;
    `

	row := s.pool.QueryRow(ctx, query, email, name)

	var u domain.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		return domain.User{}, err
	}

	return u, nil
}

// GetUserByID haalt een gebruiker op basis van ID.
func (s *UserStore) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	query := `SELECT id, email, name, created_at, updated_at FROM users WHERE id = $1`
	row := s.pool.QueryRow(ctx, query, userID)

	var u domain.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, errors.New("user not found")
		}
		return domain.User{}, err
	}

	return u, nil
}

// DeleteUser verwijdert een gebruiker en al zijn data (via ON DELETE CASCADE).
func (s *UserStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	cmdTag, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("no user found with ID " + userID.String() + " to delete")
	}
	return nil
}
