package user

import (
	"context"
	"errors"

	"agenda-automator-api/internal/database"
	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserStorer defines the interface for user store operations
type UserStorer interface {
	CreateUser(ctx context.Context, email, name string) (domain.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error)
	DeleteUser(ctx context.Context, userID uuid.UUID) error
}

// UserStore handles user-related database operations
type UserStore struct {
	db database.Querier
}

// NewUserStore creates a new UserStore
func NewUserStore(db database.Querier) UserStorer {
	return &UserStore{db: db}
}

// CreateUser maakt een nieuwe gebruiker aan in de database
func (s *UserStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	query := `
    INSERT INTO users (email, name)
    VALUES ($1, $2)
    ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
    RETURNING id, email, name, created_at, updated_at;
    `

	row := s.db.QueryRow(ctx, query, email, name)

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
	row := s.db.QueryRow(ctx, query, userID)

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
	cmdTag, err := s.db.Exec(ctx, query, userID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("no user found with ID " + userID.String() + " to delete")
	}
	return nil
}
