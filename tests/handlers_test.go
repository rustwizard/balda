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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
	"github.com/rustwizard/balda/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testAPIToken = "test-api-token"

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

// setupHandlers starts postgres + redis containers, runs migrations,
// and returns a ready Handlers instance.
func setupHandlers(t *testing.T) (*handlers.Handlers, func()) {
	t.Helper()
	ctx := context.Background()

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

	sess := session.NewService(session.Config{
		Addr:       redisAddr,
		Expiration: 30 * time.Second,
	})

	h := handlers.New(pool, sess, testAPIToken)

	cleanup := func() {
		pool.Close()
		pgCleanup()
		redisCleanup()
	}
	return h, cleanup
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
		assert.Positive(t, u.UID.Value)
		assert.Equal(t, "John", u.Firstname.Value)
		assert.Equal(t, "Smith", u.Lastname.Value)
		assert.NotEmpty(t, u.Sid.Value)
		assert.NotEmpty(t, u.Key.Value)
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
		require.True(t, ok.User.IsSet())

		u := ok.User.Value
		assert.Positive(t, u.UID.Value)
		assert.NotEmpty(t, u.Sid.Value)
	})

	t.Run("session reuse on second login", func(t *testing.T) {
		res1, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email: "auth.user@example.org", Password: "mypassword",
		})
		require.NoError(t, err)
		res2, err := h.Auth(ctx, &baldaapi.AuthRequest{
			Email: "auth.user@example.org", Password: "mypassword",
		})
		require.NoError(t, err)

		sid1 := res1.(*baldaapi.AuthResponse).User.Value.Sid.Value
		sid2 := res2.(*baldaapi.AuthResponse).User.Value.Sid.Value
		assert.Equal(t, sid1, sid2, "expected same session ID on repeated login")
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
}

func TestGetUsersStateUIDHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	// Seed a user and capture their UID.
	signupRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "State",
		Lastname:  "User",
		Email:     "state.user@example.org",
		Password:  "pass123",
	})
	require.NoError(t, err)

	uid := signupRes.(*baldaapi.SignupResponse).User.Value.UID.Value

	t.Run("existing user returns state with initial values", func(t *testing.T) {
		res, err := h.GetUsersStateUID(ctx, baldaapi.GetUsersStateUIDParams{UID: uid})
		require.NoError(t, err)

		state, isOK := res.(*baldaapi.UserState)
		require.True(t, isOK, "expected *UserState, got %T", res)

		assert.Equal(t, uid, state.UID.Value)
		assert.NotEmpty(t, state.Nickname.Value)
		assert.EqualValues(t, 5, state.Lives.Value)
		assert.EqualValues(t, 0, state.Exp.Value)
		assert.EqualValues(t, 0, state.Flags.Value)
	})

	t.Run("non-existent user returns 400", func(t *testing.T) {
		res, err := h.GetUsersStateUID(ctx, baldaapi.GetUsersStateUIDParams{UID: 9999999})
		require.NoError(t, err)

		errResp, isErr := res.(*baldaapi.ErrorResponse)
		require.True(t, isErr, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusBadRequest, errResp.Status.Value)
	})
}

// setupServer returns an httptest.Server wired with a full ogen server
// (including security middleware) plus a seeded user for auth requests.
func setupServer(t *testing.T) (srv *httptest.Server, email, password string, cleanup func()) {
	t.Helper()
	h, cleanupHandlers := setupHandlers(t)

	ogenSrv, err := baldaapi.NewServer(h, h, baldaapi.WithPathPrefix("/balda/api/v1"))
	require.NoError(t, err)

	srv = httptest.NewServer(ogenSrv)

	// Seed a user so auth requests have a valid target.
	email, password = "sec.user@example.org", "secpass"
	ctx := context.Background()
	_, err = h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Sec",
		Lastname:  "User",
		Email:     email,
		Password:  password,
	})
	require.NoError(t, err)

	cleanup = func() {
		srv.Close()
		cleanupHandlers()
	}
	return srv, email, password, cleanup
}

func postAuth(t *testing.T, srv *httptest.Server, email, password string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth",
		bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestSecurityHandlers(t *testing.T) {
	srv, email, password, cleanup := setupServer(t)
	defer cleanup()

	t.Run("HandleAPIKeyHeader: valid key returns 200", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth",
			bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", testAPIToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("HandleAPIKeyHeader: invalid key returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth",
			bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "wrong-token")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("HandleAPIKeyQueryParam: valid key returns 200", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, err := http.NewRequest(http.MethodPost,
			srv.URL+"/balda/api/v1/auth?api_key="+testAPIToken,
			bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("HandleAPIKeyQueryParam: invalid key returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, err := http.NewRequest(http.MethodPost,
			srv.URL+"/balda/api/v1/auth?api_key=bad-key",
			bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("no key returns 401", func(t *testing.T) {
		resp := postAuth(t, srv, email, password)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
