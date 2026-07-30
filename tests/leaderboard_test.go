package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rustwizard/balda/internal/centrifugo"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLeaderboard returns a handler, storage and cleanup for leaderboard tests.
func setupLeaderboard(t *testing.T) (*handlers.Handlers, *storage.Balda, *redis.Client, func()) {
	t.Helper()
	ctx := context.Background()
	core := setupCore(ctx, t)
	cf := centrifugo.NewClient("http://localhost:8000/api", "test-key")
	h := handlers.New(core.svc, core.pres, testJWTSecret, cf, "test-secret", true, "")
	return h, core.s, core.rdb, core.cleanup
}

// seedLeaderboardPlayer inserts a user + player_state row with specific rating/exp and updated_at.
func seedLeaderboardPlayer(ctx context.Context, t *testing.T, s *storage.Balda, email string, rating int, exp int64, updatedAt time.Time) uuid.UUID {
	t.Helper()
	playerID := uuid.New()
	var userID int64
	err := s.Pool().QueryRow(ctx,
		`INSERT INTO users (first_name, last_name, email, hash_password) VALUES ('Test', 'User', $1, 'x') RETURNING user_id`,
		email,
	).Scan(&userID)
	require.NoError(t, err)

	_, err = s.Pool().Exec(ctx,
		`INSERT INTO player_state (user_id, player_id, nickname, exp, rating, flags, lives, updated_at) VALUES ($1, $2, $3, $4, $5, 0, 5, $6)`,
		userID, playerID, email, exp, rating, updatedAt,
	)
	require.NoError(t, err)
	return playerID
}

func TestGetLeaderboard(t *testing.T) {
	ctx := context.Background()
	h, s, _, cleanup := setupLeaderboard(t)
	defer cleanup()

	now := time.Now().UTC()
	// Active players within the period.
	p1 := seedLeaderboardPlayer(ctx, t, s, "lb.p1@example.org", 1500, 3000, now.Add(-1*time.Hour))
	p2 := seedLeaderboardPlayer(ctx, t, s, "lb.p2@example.org", 1400, 4000, now.Add(-2*time.Hour))
	p3 := seedLeaderboardPlayer(ctx, t, s, "lb.p3@example.org", 1300, 5000, now.Add(-3*time.Hour))
	// Inactive player outside the period.
	seedLeaderboardPlayer(ctx, t, s, "lb.old@example.org", 2000, 9000, now.Add(-40*24*time.Hour))

	t.Run("top by rating", func(t *testing.T) {
		res, err := h.GetLeaderboard(ctx, baldaapi.GetLeaderboardParams{
			Period: baldaapi.GetLeaderboardPeriodWeek,
			Sort:   baldaapi.NewOptGetLeaderboardSort(baldaapi.GetLeaderboardSortRating),
			Limit:  baldaapi.NewOptInt(10),
		})
		require.NoError(t, err)

		ok, isOK := res.(*baldaapi.LeaderboardResponse)
		require.True(t, isOK, "expected *LeaderboardResponse, got %T", res)
		require.Len(t, ok.Players, 3)

		assert.Equal(t, 1, ok.Players[0].Rank.Value)
		assert.Equal(t, p1, ok.Players[0].UID.Value)
		assert.EqualValues(t, 1500, ok.Players[0].Rating.Value)

		assert.Equal(t, 2, ok.Players[1].Rank.Value)
		assert.Equal(t, p2, ok.Players[1].UID.Value)

		assert.Equal(t, 3, ok.Players[2].Rank.Value)
		assert.Equal(t, p3, ok.Players[2].UID.Value)
	})

	t.Run("top by exp", func(t *testing.T) {
		res, err := h.GetLeaderboard(ctx, baldaapi.GetLeaderboardParams{
			Period: baldaapi.GetLeaderboardPeriodWeek,
			Sort:   baldaapi.NewOptGetLeaderboardSort(baldaapi.GetLeaderboardSortExp),
			Limit:  baldaapi.NewOptInt(10),
		})
		require.NoError(t, err)

		ok, isOK := res.(*baldaapi.LeaderboardResponse)
		require.True(t, isOK, "expected *LeaderboardResponse, got %T", res)
		require.Len(t, ok.Players, 3)

		assert.Equal(t, p3, ok.Players[0].UID.Value)
		assert.Equal(t, p2, ok.Players[1].UID.Value)
		assert.Equal(t, p1, ok.Players[2].UID.Value)
	})

	t.Run("limit caps result", func(t *testing.T) {
		res, err := h.GetLeaderboard(ctx, baldaapi.GetLeaderboardParams{
			Period: baldaapi.GetLeaderboardPeriodWeek,
			Sort:   baldaapi.NewOptGetLeaderboardSort(baldaapi.GetLeaderboardSortRating),
			Limit:  baldaapi.NewOptInt(2),
		})
		require.NoError(t, err)

		ok, isOK := res.(*baldaapi.LeaderboardResponse)
		require.True(t, isOK, "expected *LeaderboardResponse, got %T", res)
		require.Len(t, ok.Players, 2)
	})
}

func TestLeaderboardCache(t *testing.T) {
	ctx := context.Background()
	h, s, rdb, cleanup := setupLeaderboard(t)
	defer cleanup()

	now := time.Now().UTC()
	p1 := seedLeaderboardPlayer(ctx, t, s, "lb.cache@example.org", 1500, 3000, now.Add(-1*time.Hour))

	req := baldaapi.GetLeaderboardParams{
		Period: baldaapi.GetLeaderboardPeriodWeek,
		Sort:   baldaapi.NewOptGetLeaderboardSort(baldaapi.GetLeaderboardSortRating),
		Limit:  baldaapi.NewOptInt(10),
	}

	// First call should populate cache.
	_, err := h.GetLeaderboard(ctx, req)
	require.NoError(t, err)

	key := "leaderboard:week:rating:10"
	cached, err := rdb.Get(ctx, key).Result()
	require.NoError(t, err)
	assert.Contains(t, cached, p1.String())

	ttl, err := rdb.PTTL(ctx, key).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))

	// Second call should return cached data (we verify by checking the same key still exists).
	_, err = h.GetLeaderboard(ctx, req)
	require.NoError(t, err)
	cached2, err := rdb.Get(ctx, key).Result()
	require.NoError(t, err)
	assert.Equal(t, cached, cached2)
}

func TestLeaderboardHTTP(t *testing.T) {
	ctx := context.Background()
	h, s, _, cleanup := setupLeaderboard(t)
	defer cleanup()

	now := time.Now().UTC()
	seedLeaderboardPlayer(ctx, t, s, "lb.http@example.org", 1500, 3000, now.Add(-1*time.Hour))

	srv, err := baldaapi.NewServer(h, h, baldaapi.WithPathPrefix("/balda/api/v1"))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/balda/api/v1/leaderboard?period=week&sort=rating&limit=10", http.NoBody)
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "lb.http")

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/balda/api/v1/leaderboard?period=invalid", http.NoBody)
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
