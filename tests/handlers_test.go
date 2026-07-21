package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rustwizard/balda/internal/achievements"
	"github.com/rustwizard/balda/internal/auth"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/leaderboard"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/rustwizard/balda/internal/presence"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/rustwizard/balda/internal/service"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/rustwizard/balda/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startRedis(ctx context.Context, t *testing.T) (addr string, cleanup func()) {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	addr = fmt.Sprintf("%s:%s", host, port.Port())
	cleanup = func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate redis container: %v", err)
		}
	}
	return addr, cleanup
}

type coreSetup struct {
	svc     *service.Balda
	pres    *presence.Service
	lby     *lobby.Lobby
	s       *storage.Balda
	rdb     *redis.Client
	cleanup func()
}

// testJWTSecret signs access tokens in tests; authCtx parses with the same secret.
const testJWTSecret = "test-jwt-secret-at-least-32-bytes!!"

// authCtx parses a raw access token and returns a context carrying its claims,
// mirroring what HandleBearerAuth does in production for direct handler calls.
func authCtx(t *testing.T, token string) context.Context {
	t.Helper()
	claims, err := auth.ParseAccessToken(token, testJWTSecret)
	require.NoError(t, err)
	return auth.WithClaims(context.Background(), claims)
}

// signupCtx signs up a user via the handler and returns a context carrying their
// JWT claims plus the player UUID (pid), for direct handler-call tests.
func signupCtx(t *testing.T, h *handlers.Handlers, email string) (context.Context, uuid.UUID) {
	t.Helper()
	res, err := h.Signup(context.Background(), &baldaapi.SignupRequest{
		Firstname: "Test", Lastname: "User", Email: email, Password: "secret",
	})
	require.NoError(t, err)
	resp, ok := res.(*baldaapi.SignupResponse)
	require.True(t, ok, "expected *SignupResponse, got %T", res)
	return authCtx(t, resp.AccessToken.Value), resp.User.Value.UID.Value
}

// setupCore starts postgres + redis containers, runs migrations, and returns
// the shared service layer. Call cleanup when done.
func setupCore(ctx context.Context, t *testing.T) *coreSetup {
	t.Helper()

	pgc, pgCleanup := startPG(ctx, t)

	connStr, err := pgc.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, os.Setenv("MIGRATION_CONN_STRING", connStr))

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	// pgcrypto is required for crypt() / gen_salt() used in auth queries.
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto")
	require.NoError(t, err)

	_, err = migrations.Migrate(10 * time.Second)
	require.NoError(t, err)

	redisAddr, redisCleanup := startRedis(ctx, t)

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	pres := presence.NewService(presence.Config{}, rdb)

	lby := lobby.New(func(_ context.Context, _ string, players []*game.Player, n game.Notifier) (*game.Game, error) {
		return game.NewGameWithWord(players, "масло", n)
	})
	mm := matchmaking.New(matchmaking.DefaultConfig(), func(players []*game.Player) error {
		_, err := lby.StartGame(ctx, players, &game.NoopNotifier{})
		return err
	})

	s := storage.New(pool, 10*time.Second)
	lb := leaderboard.NewService(s, rdb, 5*time.Minute)
	achSvc := achievements.NewService(s.LoadAchievementDefinitions)
	require.NoError(t, achSvc.Load(ctx))
	svc := service.New(lby, mm, s, lb, achSvc)

	return &coreSetup{
		svc:  svc,
		pres: pres,
		lby:  lby,
		s:    s,
		rdb:  rdb,
		cleanup: func() {
			pool.Close()
			pgCleanup()
			redisCleanup()
		},
	}
}

func setupHandlers(t *testing.T) (*handlers.Handlers, func()) {
	t.Helper()
	ctx := context.Background()
	core := setupCore(ctx, t)
	cf := centrifugo.NewClient("http://localhost:8000/api", "test-key")
	h := handlers.New(core.svc, core.pres, testJWTSecret, cf, "test-secret")
	return h, core.cleanup
}

func TestSignupHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		res, err := h.Signup(ctx, &baldaapi.SignupRequest{
			Firstname: "John",
			Lastname:  "Smith",
			Email:     "john.smith@example.org",
			Password:  "secret",
		})
		require.NoError(t, err)

		ok, isOK := res.(*baldaapi.SignupResponse)
		require.True(t, isOK, "expected *SignupResponse, got %T", res)
		require.True(t, ok.User.IsSet())

		u := ok.User.Value
		assert.NotEqual(t, uuid.UUID{}, u.UID.Value)
		assert.Equal(t, "John", u.Firstname.Value)
		assert.Equal(t, "Smith", u.Lastname.Value)
		assert.NotEmpty(t, ok.AccessToken.Value)
		assert.NotEmpty(t, ok.RefreshToken.Value)
		assert.Equal(t, "Bearer", ok.TokenType.Value)
		assert.Positive(t, ok.ExpiresIn.Value)
	})

	t.Run("duplicate email returns error", func(t *testing.T) {
		res, err := h.Signup(ctx, &baldaapi.SignupRequest{
			Firstname: "Jane",
			Lastname:  "Doe",
			Email:     "john.smith@example.org", // already registered above
			Password:  "other",
		})
		require.NoError(t, err)

		errResp, isErr := res.(*baldaapi.ErrorResponse)
		require.True(t, isErr, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusBadRequest, errResp.Status.Value)
	})
}

func TestAuthHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	// Seed a user.
	_, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Auth",
		Lastname:  "User",
		Email:     "auth.user@example.org",
		Password:  "mypassword",
	})
	require.NoError(t, err)

	t.Run("correct credentials", func(t *testing.T) {
		res, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email:    "auth.user@example.org",
			Password: "mypassword",
		})
		require.NoError(t, err)

		ok, isOK := res.(*baldaapi.AuthResponse)
		require.True(t, isOK, "expected *AuthResponse, got %T", res)
		require.True(t, ok.Player.IsSet())

		u := ok.Player.Value
		assert.NotEqual(t, uuid.UUID{}, u.UID.Value)
		assert.NotEmpty(t, ok.AccessToken.Value, "expected access token")
		assert.NotEmpty(t, ok.RefreshToken.Value, "expected refresh token")
		assert.Equal(t, "Bearer", ok.TokenType.Value)
	})

	t.Run("each login issues a fresh token pair", func(t *testing.T) {
		res1, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email: "auth.user@example.org", Password: "mypassword",
		})
		require.NoError(t, err)
		res2, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email: "auth.user@example.org", Password: "mypassword",
		})
		require.NoError(t, err)

		rt1 := res1.(*baldaapi.AuthResponse).RefreshToken.Value
		rt2 := res2.(*baldaapi.AuthResponse).RefreshToken.Value
		assert.NotEmpty(t, rt1)
		assert.NotEqual(t, rt1, rt2, "expected a new refresh token on each login")
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		res, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email:    "auth.user@example.org",
			Password: "wrongpassword",
		})
		require.NoError(t, err)

		errResp, isErr := res.(*baldaapi.ErrorResponse)
		require.True(t, isErr, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("unknown email returns 401", func(t *testing.T) {
		res, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email:    "nobody@example.org",
			Password: "whatever",
		})
		require.NoError(t, err)

		errResp, isErr := res.(*baldaapi.ErrorResponse)
		require.True(t, isErr, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("no active_game when player has no game", func(t *testing.T) {
		res, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email:    "auth.user@example.org",
			Password: "mypassword",
		})
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.AuthResponse)
		require.True(t, ok)
		assert.False(t, resp.ActiveGame.IsSet(), "expected active_game to be absent")
	})

	t.Run("active_game returned after joining an in_progress game", func(t *testing.T) {
		// Sign up two players and start a game.
		creatorCtx, _ := signupCtx(t, h, "reconnect.creator@example.org")
		joinerCtx, _ := signupCtx(t, h, "reconnect.joiner@example.org")

		createRes, err := h.CreateGame(creatorCtx)
		require.NoError(t, err)
		gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

		_, err = h.JoinGame(joinerCtx, baldaapi.JoinGameParams{ID: gameID})
		require.NoError(t, err)

		// Creator re-authenticates — should get active_game.
		res, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email:    "reconnect.creator@example.org",
			Password: "secret",
		})
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.AuthResponse)
		require.True(t, ok)
		require.True(t, resp.ActiveGame.IsSet(), "expected active_game to be set")

		ag := resp.ActiveGame.Value
		assert.Equal(t, gameID, ag.GameID.Value)
		assert.NotEmpty(t, ag.GameToken.Value)
		assert.Len(t, ag.Board, 5)
		assert.NotEmpty(t, ag.CurrentTurnUID.Value)
		assert.Equal(t, baldaapi.GameStatusInProgress, ag.Status.Value)
		assert.Len(t, ag.Players, 2)
	})
}

// TestAuthFlow verifies the full browser-facing auth flow over HTTP:
// signup and /auth are public and return a JWT access token, and that token
// authorizes protected endpoints via the Authorization: Bearer header.
func TestAuthFlow(t *testing.T) {
	srv, _, cleanup := setupServer(t)
	defer cleanup()

	const flowEmail = "flow.user@example.org"
	const flowPassword = "flowpass"

	// Sign up via HTTP — no token required.
	signupBody, err := json.Marshal(map[string]string{
		"firstname": "Flow",
		"lastname":  "User",
		"email":     flowEmail,
		"password":  flowPassword,
	})
	require.NoError(t, err)
	signupReq, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/signup", bytes.NewReader(signupBody))
	require.NoError(t, err)
	signupReq.Header.Set("Content-Type", "application/json")
	signupResp, err := http.DefaultClient.Do(signupReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, signupResp.StatusCode)
	var signupOut struct {
		User struct {
			UID string `json:"uid"`
		} `json:"user"`
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(signupResp.Body).Decode(&signupOut))
	signupResp.Body.Close()
	require.NotEmpty(t, signupOut.User.UID)
	require.NotEmpty(t, signupOut.AccessToken, "signup should return an access token")

	// Authenticate via HTTP — public endpoint.
	authBody, err := json.Marshal(map[string]string{
		"email":    flowEmail,
		"password": flowPassword,
	})
	require.NoError(t, err)
	authReq, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth", bytes.NewReader(authBody))
	require.NoError(t, err)
	authReq.Header.Set("Content-Type", "application/json")
	authResp, err := http.DefaultClient.Do(authReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, authResp.StatusCode)
	var authOut struct {
		Player struct {
			UID string `json:"uid"`
		} `json:"player"`
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(authResp.Body).Decode(&authOut))
	authResp.Body.Close()
	require.NotEmpty(t, authOut.AccessToken, "auth should return an access token")
	assert.Equal(t, signupOut.User.UID, authOut.Player.UID)

	// The returned access token must work for a protected endpoint.
	listReq, err := http.NewRequest(http.MethodGet, srv.URL+"/balda/api/v1/games", http.NoBody)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+authOut.AccessToken)
	listResp, err := http.DefaultClient.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "access token should authorize protected endpoints")

	// Auth with wrong password must still reject.
	badAuthBody, err := json.Marshal(map[string]string{
		"email":    flowEmail,
		"password": "wrongpassword",
	})
	require.NoError(t, err)
	badAuthReq, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth", bytes.NewReader(badAuthBody))
	require.NoError(t, err)
	badAuthReq.Header.Set("Content-Type", "application/json")
	badAuthResp, err := http.DefaultClient.Do(badAuthReq)
	require.NoError(t, err)
	defer badAuthResp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, badAuthResp.StatusCode)
}

func TestGetUsersStateUIDHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx, uid := signupCtx(t, h, "state.user@example.org")

	t.Run("existing user returns state with initial values", func(t *testing.T) {
		res, err := h.GetPlayerStateUID(ctx, baldaapi.GetPlayerStateUIDParams{UID: uid})
		require.NoError(t, err)

		state, isOK := res.(*baldaapi.PlayerState)
		require.True(t, isOK, "expected *PlayerState, got %T", res)

		assert.Equal(t, uid, state.UID.Value)
		assert.NotEmpty(t, state.Nickname.Value)
		assert.EqualValues(t, 5, state.Lives.Value)
		assert.EqualValues(t, 0, state.Exp.Value)
		assert.EqualValues(t, 0, state.Flags.Value)
		assert.False(t, state.GameID.IsSet(), "expected GameID to be unset for player not in a game")
	})

	t.Run("non-existent user returns 400", func(t *testing.T) {
		fakeUID := uuid.New()
		fakeCtx := auth.WithClaims(context.Background(), &auth.Claims{PlayerID: fakeUID})
		res, err := h.GetPlayerStateUID(fakeCtx, baldaapi.GetPlayerStateUIDParams{UID: fakeUID})
		require.NoError(t, err)

		errResp, isErr := res.(*baldaapi.GetPlayerStateUIDBadRequest)
		require.True(t, isErr, "expected *GetPlayerStateUIDBadRequest, got %T", res)
		assert.Equal(t, http.StatusBadRequest, errResp.Status.Value)
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		res, err := h.GetPlayerStateUID(context.Background(), baldaapi.GetPlayerStateUIDParams{UID: uid})
		require.NoError(t, err)

		unauth, isUnauthorized := res.(*baldaapi.GetPlayerStateUIDUnauthorized)
		require.True(t, isUnauthorized, "expected *GetPlayerStateUIDUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, unauth.Status.Value)
	})

	t.Run("another player's state returns 403", func(t *testing.T) {
		_, otherUID := signupCtx(t, h, "state.other@example.org")
		res, err := h.GetPlayerStateUID(ctx, baldaapi.GetPlayerStateUIDParams{UID: otherUID})
		require.NoError(t, err)

		forbidden, isForbidden := res.(*baldaapi.GetPlayerStateUIDForbidden)
		require.True(t, isForbidden, "expected *GetPlayerStateUIDForbidden, got %T", res)
		assert.Equal(t, http.StatusForbidden, forbidden.Status.Value)
	})
}

// setupFull is like setupHandlers but also returns the lobby for direct manipulation in tests.
func setupFull(t *testing.T) (*handlers.Handlers, *lobby.Lobby, func()) {
	t.Helper()
	ctx := context.Background()
	core := setupCore(ctx, t)
	cf := centrifugo.NewClient("http://localhost:8000/api", "test-key")
	h := handlers.New(core.svc, core.pres, testJWTSecret, cf, "test-secret")
	return h, core.lby, core.cleanup
}

func TestGetPlayerStateUID_GameID(t *testing.T) {
	h, lby, cleanup := setupFull(t)
	defer cleanup()

	ctx := context.Background()

	// Seed two users so we can form a valid game.
	ctx1, uid1 := signupCtx(t, h, "gps.one@example.org")
	ctx2, uid2 := signupCtx(t, h, "gps.two@example.org")

	// Start a game in the lobby so both players are in an active game.
	players := []*game.Player{
		{ID: uid1.String(), Exp: 100, Type: game.PlayerTypeHuman},
		{ID: uid2.String(), Exp: 200, Type: game.PlayerTypeHuman},
	}
	_, err := lby.StartGame(ctx, players, &game.NoopNotifier{})
	require.NoError(t, err)

	t.Run("player in active game has GameID set", func(t *testing.T) {
		res, err := h.GetPlayerStateUID(ctx1, baldaapi.GetPlayerStateUIDParams{UID: uid1})
		require.NoError(t, err)

		state, isOK := res.(*baldaapi.PlayerState)
		require.True(t, isOK, "expected *PlayerState, got %T", res)

		assert.True(t, state.GameID.IsSet(), "expected GameID to be set")
		assert.NotEqual(t, uuid.UUID{}, state.GameID.Value)
		assert.Equal(t, uid1, state.UID.Value)
	})

	t.Run("second player in same game also has GameID set", func(t *testing.T) {
		res, err := h.GetPlayerStateUID(ctx2, baldaapi.GetPlayerStateUIDParams{UID: uid2})
		require.NoError(t, err)

		state, isOK := res.(*baldaapi.PlayerState)
		require.True(t, isOK, "expected *PlayerState, got %T", res)

		assert.True(t, state.GameID.IsSet(), "expected GameID to be set for second player")
	})

	t.Run("both players share the same GameID", func(t *testing.T) {
		res1, err := h.GetPlayerStateUID(ctx1, baldaapi.GetPlayerStateUIDParams{UID: uid1})
		require.NoError(t, err)
		res2, err := h.GetPlayerStateUID(ctx2, baldaapi.GetPlayerStateUIDParams{UID: uid2})
		require.NoError(t, err)

		state1 := res1.(*baldaapi.PlayerState)
		state2 := res2.(*baldaapi.PlayerState)

		assert.Equal(t, state1.GameID.Value, state2.GameID.Value, "both players should share the same game ID")
	})

	t.Run("player not in any game has no GameID", func(t *testing.T) {
		ctx3, uid3 := signupCtx(t, h, "gps.three@example.org")

		res, err := h.GetPlayerStateUID(ctx3, baldaapi.GetPlayerStateUIDParams{UID: uid3})
		require.NoError(t, err)

		state, isOK := res.(*baldaapi.PlayerState)
		require.True(t, isOK, "expected *PlayerState, got %T", res)
		assert.False(t, state.GameID.IsSet(), "expected GameID to be unset for player not in a game")
	})
}

func TestPingHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	playerCtx, _ := signupCtx(t, h, "ping.user@example.org")

	t.Run("authenticated ping returns 204", func(t *testing.T) {
		res, err := h.Ping(playerCtx, baldaapi.PingParams{XRequestID: 1})
		require.NoError(t, err)

		_, ok := res.(*baldaapi.PingNoContent)
		require.True(t, ok, "expected *PingNoContent, got %T", res)
	})

	t.Run("x-request-id is echoed back", func(t *testing.T) {
		const reqID int64 = 42
		res, err := h.Ping(playerCtx, baldaapi.PingParams{XRequestID: reqID})
		require.NoError(t, err)

		pong := res.(*baldaapi.PingNoContent)
		assert.Equal(t, reqID, pong.XRequestID.Value)
	})

	t.Run("x-server-time reflects current time", func(t *testing.T) {
		before := time.Now().UnixMilli()
		res, err := h.Ping(playerCtx, baldaapi.PingParams{XRequestID: 1})
		require.NoError(t, err)

		pong := res.(*baldaapi.PingNoContent)
		assert.GreaterOrEqual(t, pong.XServerTime.Value, before)
	})

	t.Run("missing claims returns 401", func(t *testing.T) {
		res, err := h.Ping(context.Background(), baldaapi.PingParams{XRequestID: 1})
		require.NoError(t, err)

		errResp, ok := res.(*baldaapi.ErrorResponse)
		require.True(t, ok, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})
}

func TestPingHTTP(t *testing.T) {
	srv, token, cleanup := setupServer(t)
	defer cleanup()

	pingURL := srv.URL + "/balda/api/v1/session/ping"

	t.Run("missing token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, pingURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-Request-ID", "1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, pingURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
		req.Header.Set("X-Request-ID", "1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("valid token returns 204 with response headers", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, pingURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Request-ID", "42")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "42", resp.Header.Get("X-Request-Id"))
		assert.NotEmpty(t, resp.Header.Get("X-Server-Time"))
	})
}

// setupServer returns an httptest.Server wired with a full ogen server
// (including security middleware) plus a seeded user for auth requests.
// token is the seeded user's JWT access token for the Authorization header.
func setupServer(t *testing.T) (srv *httptest.Server, token string, cleanup func()) {
	t.Helper()
	h, cleanupHandlers := setupHandlers(t)

	ogenSrv, err := baldaapi.NewServer(h, h,
		baldaapi.WithPathPrefix("/balda/api/v1"),
		baldaapi.WithErrorHandler(handlers.ErrorHandler),
	)
	require.NoError(t, err)

	srv = httptest.NewServer(ogenSrv)

	// Seed a user so auth requests have a valid target.
	ctx := context.Background()
	res, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Sec",
		Lastname:  "User",
		Email:     "sec.user@example.org",
		Password:  "secpass",
	})
	require.NoError(t, err)
	signupRes, ok := res.(*baldaapi.SignupResponse)
	require.True(t, ok)
	token = signupRes.AccessToken.Value

	cleanup = func() {
		srv.Close()
		cleanupHandlers()
	}
	return srv, token, cleanup
}

// postSignup signs up a new user via HTTP and returns their JWT access token.
func postSignup(t *testing.T, srv *httptest.Server, email, password string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"firstname": "Test", "lastname": "User", "email": email, "password": password,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/signup", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.AccessToken)
	return out.AccessToken
}

func TestSecurityHandlers(t *testing.T) {
	srv, token, cleanup := setupServer(t)
	defer cleanup()

	gamesURL := srv.URL + "/balda/api/v1/games"

	t.Run("valid bearer token returns 200", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid bearer token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer garbage.token.value")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("missing token returns 401 in ErrorResponse shape", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Body must follow the ErrorResponse schema, not ogen's raw error_message,
		// and must not leak operation/security internals.
		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.EqualValues(t, http.StatusUnauthorized, body["status"])
		assert.NotEmpty(t, body["message"])
		assert.NotContains(t, body, "error_message")
		assert.NotContains(t, body["message"], "security")
	})
}
