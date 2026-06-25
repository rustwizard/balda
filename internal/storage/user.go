package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor for password hashing.
const bcryptCost = 12

// ErrInvalidCredentials is returned by AuthUser when the email is unknown or the
// password does not match. Callers should map it to 401, distinct from DB errors (500).
var ErrInvalidCredentials = errors.New("storage: invalid credentials")

// UserAuth holds the data returned by a successful credential check.
type UserAuth struct {
	UID       int64
	Firstname string
	Lastname  string
	PlayerID  uuid.UUID
	Exp       int64
	Role      string
}

// UserForToken holds the fields needed to mint an access token for an existing user.
type UserForToken struct {
	PlayerID uuid.UUID
	Role     string
}

// UserCreated holds the data returned after a successful signup.
type UserCreated struct {
	UID      int64
	PlayerID uuid.UUID
	Role     string
}

// AuthUser verifies email/password and returns the user's identity.
func (b *Balda) AuthUser(ctx context.Context, email, password string) (UserAuth, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var u UserAuth
	var hash string
	err := b.db.QueryRow(ctx, `
		SELECT u.user_id, u.first_name, u.last_name, ps.player_id, COALESCE(ps.exp, 0), u.role, u.hash_password
		FROM users u
		JOIN player_state ps ON ps.user_id = u.user_id
		WHERE u.email = $1
	`, email).Scan(&u.UID, &u.Firstname, &u.Lastname, &u.PlayerID, &u.Exp, &u.Role, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserAuth{}, ErrInvalidCredentials
	}
	if err != nil {
		return UserAuth{}, fmt.Errorf("auth user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return UserAuth{}, ErrInvalidCredentials
	}
	return u, nil
}

// GetUserForToken returns the player UUID and role for an existing user, used
// when minting a fresh access token during refresh.
func (b *Balda) GetUserForToken(ctx context.Context, uid int64) (UserForToken, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var u UserForToken
	err := b.db.QueryRow(ctx, `
		SELECT ps.player_id, u.role
		FROM users u
		JOIN player_state ps ON ps.user_id = u.user_id
		WHERE u.user_id = $1
	`, uid).Scan(&u.PlayerID, &u.Role)
	if err != nil {
		return UserForToken{}, fmt.Errorf("get user for token: %w", err)
	}
	return u, nil
}

// CreateUser inserts a new user and their player_state in a single transaction.
func (b *Balda) CreateUser(ctx context.Context, firstname, lastname, email, password, nickname string) (UserCreated, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return UserCreated{}, fmt.Errorf("create user: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return UserCreated{}, fmt.Errorf("create user: hash password: %w", err)
	}

	var created UserCreated
	err = tx.QueryRow(ctx,
		`INSERT INTO users(first_name, last_name, email, hash_password)
		 VALUES($1, $2, $3, $4) RETURNING user_id, role`,
		firstname, lastname, email, string(hash),
	).Scan(&created.UID, &created.Role)
	if err != nil {
		return UserCreated{}, fmt.Errorf("create user: insert users: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO player_state(user_id, nickname, exp, rating, flags, lives)
		 VALUES($1, $2, $3, $4, $5, $6) RETURNING player_id`,
		created.UID, nickname, 0, DefaultRating, 0, 5,
	).Scan(&created.PlayerID)
	if err != nil {
		return UserCreated{}, fmt.Errorf("create user: insert player_state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return UserCreated{}, fmt.Errorf("create user: commit: %w", err)
	}
	return created, nil
}
