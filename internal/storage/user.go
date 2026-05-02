package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// UserAuth holds the data returned by a successful credential check.
type UserAuth struct {
	UID       int64
	Firstname string
	Lastname  string
	PlayerID  uuid.UUID
	Exp       int64
}

// UserCreated holds the data returned after a successful signup.
type UserCreated struct {
	UID      int64
	APIKey   string
	PlayerID uuid.UUID
}

// AuthUser verifies email/password and returns the user's identity.
func (b *Balda) AuthUser(ctx context.Context, email, password string) (UserAuth, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var u UserAuth
	err := b.db.QueryRow(ctx, `
		SELECT u.user_id, u.first_name, u.last_name, ps.player_id, COALESCE(ps.exp, 0)
		FROM users u
		JOIN player_state ps ON ps.user_id = u.user_id
		WHERE u.email = $1 AND u.hash_password = crypt($2, u.hash_password)
	`, email, password).Scan(&u.UID, &u.Firstname, &u.Lastname, &u.PlayerID, &u.Exp)
	if err != nil {
		return UserAuth{}, fmt.Errorf("auth user: %w", err)
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

	var created UserCreated
	err = tx.QueryRow(ctx,
		`INSERT INTO users(first_name, last_name, email, hash_password)
		 VALUES($1, $2, $3, crypt($4, gen_salt('bf', 8))) RETURNING user_id, api_key`,
		firstname, lastname, email, password,
	).Scan(&created.UID, &created.APIKey)
	if err != nil {
		return UserCreated{}, fmt.Errorf("create user: insert users: %w", err)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO player_state(user_id, nickname, exp, flags, lives)
		 VALUES($1, $2, $3, $4, $5) RETURNING player_id`,
		created.UID, nickname, 0, 0, 5,
	).Scan(&created.PlayerID)
	if err != nil {
		return UserCreated{}, fmt.Errorf("create user: insert player_state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return UserCreated{}, fmt.Errorf("create user: commit: %w", err)
	}
	return created, nil
}
