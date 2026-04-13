package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gocf "github.com/centrifugal/centrifuge-go"
	"github.com/rustwizard/balda/internal/centrifugo"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testCentrifugoSecret = "test-centrifugo-secret"
	testCentrifugoAPIKey = "test-centrifugo-api-key"
)

func startCentrifugo(ctx context.Context, t *testing.T) (apiURL, wsURL string, cleanup func()) {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "centrifugo/centrifugo:v6",
			ExposedPorts: []string{"8000/tcp"},
			Env: map[string]string{
				"CENTRIFUGO_CLIENT_TOKEN_HMAC_SECRET_KEY": testCentrifugoSecret,
				"CENTRIFUGO_HTTP_API_KEY":                 testCentrifugoAPIKey,
				"CENTRIFUGO_CLIENT_ALLOWED_ORIGINS":       "*",
				"CENTRIFUGO_HEALTH_ENABLED":               "true",
				"CENTRIFUGO_CHANNEL_NAMESPACES":           `[{"name":"game"}]`,
			},
			Cmd:        []string{"centrifugo"},
			WaitingFor: wait.ForHTTP("/health").WithPort("8000/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "8000")
	require.NoError(t, err)

	base := fmt.Sprintf("%s:%s", host, port.Port())
	apiURL = "http://" + base + "/api"
	wsURL = "ws://" + base + "/connection/websocket"

	cleanup = func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate centrifugo container: %v", err)
		}
	}
	return
}

func setupHandlersWithCentrifugo(ctx context.Context, t *testing.T, cfAPIURL string) (*handlers.Handlers, func()) {
	t.Helper()
	core := setupCore(ctx, t)
	cf := centrifugo.NewClient(cfAPIURL, testCentrifugoAPIKey)
	h := handlers.New(core.svc, core.sess, testAPIToken, cf, testCentrifugoSecret)
	return h, core.cleanup
}

func subscribeToChannel(t *testing.T, wsURL, channel, connToken, subToken string) <-chan []byte {
	t.Helper()

	ch := make(chan []byte, 4)
	subscribed := make(chan struct{})

	client := gocf.NewJsonClient(wsURL, gocf.Config{Token: connToken})
	client.OnError(func(e gocf.ErrorEvent) {
		t.Logf("centrifuge client error: %v", e.Error)
	})
	client.OnDisconnected(func(e gocf.DisconnectedEvent) {
		t.Logf("centrifuge disconnected: code=%d reason=%s", e.Code, e.Reason)
	})

	sub, err := client.NewSubscription(channel, gocf.SubscriptionConfig{Token: subToken})
	require.NoError(t, err)

	sub.OnPublication(func(e gocf.PublicationEvent) {
		data := make([]byte, len(e.Data))
		copy(data, e.Data)
		ch <- data
	})
	sub.OnSubscribed(func(_ gocf.SubscribedEvent) {
		close(subscribed)
	})
	sub.OnError(func(e gocf.SubscriptionErrorEvent) {
		t.Logf("centrifuge subscription error on %s: %v", channel, e.Error)
	})

	require.NoError(t, sub.Subscribe())
	require.NoError(t, client.Connect())

	select {
	case <-subscribed:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for subscription to be established")
	}

	t.Cleanup(func() { client.Close() })

	return ch
}

func TestCreateGame_PublishesEvGameCreated(t *testing.T) {
	ctx := context.Background()

	cfAPIURL, wsURL, cfCleanup := startCentrifugo(ctx, t)
	defer cfCleanup()

	h, cleanup := setupHandlersWithCentrifugo(ctx, t, cfAPIURL)
	defer cleanup()

	srv, err := baldaapi.NewServer(h, h, baldaapi.WithPathPrefix("/balda/api/v1"))
	require.NoError(t, err)

	// signup to get a session
	signupBody, _ := json.Marshal(map[string]string{
		"firstname": "Event", "lastname": "Tester",
		"email": "event@test.com", "password": "pass",
	})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, newReq(t, http.MethodPost, "/balda/api/v1/signup", signupBody))
	require.Equal(t, http.StatusOK, w.Code)

	var signupResp baldaapi.SignupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &signupResp))
	sid := signupResp.User.Value.Sid.Value
	cfToken := signupResp.CentrifugoToken.Value
	lobbyToken := signupResp.LobbyToken.Value

	// subscribe to lobby before creating the game
	eventCh := subscribeToChannel(t, wsURL, centrifugo.ChannelLobby, cfToken, lobbyToken)

	// create game
	w = httptest.NewRecorder()
	req := newReq(t, http.MethodPost, "/balda/api/v1/games", nil)
	req.Header.Set("X-API-Key", testAPIToken)
	req.Header.Set("X-API-Session", sid)
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// wait for EvGameCreated on lobby channel
	select {
	case data := <-eventCh:
		var ev centrifugo.EvGameCreated
		require.NoError(t, json.Unmarshal(data, &ev))
		assert.Equal(t, "game_created", ev.Type)
		assert.Equal(t, "waiting", ev.Status)
		assert.NotEmpty(t, ev.GameID)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for EvGameCreated")
	}
}

func TestJoinGame_PublishesEvGameStarted(t *testing.T) {
	ctx := context.Background()

	cfAPIURL, wsURL, cfCleanup := startCentrifugo(ctx, t)
	defer cfCleanup()

	h, cleanup := setupHandlersWithCentrifugo(ctx, t, cfAPIURL)
	defer cleanup()

	srv, err := baldaapi.NewServer(h, h, baldaapi.WithPathPrefix("/balda/api/v1"))
	require.NoError(t, err)

	// signup player 1 and player 2
	p1Sid, p1CfToken, p1LobbyToken := signupPlayer(t, srv, "player1@test.com")
	p2Sid, _, _ := signupPlayer(t, srv, "player2@test.com")

	// player 1 creates a game
	w := httptest.NewRecorder()
	req := newReq(t, http.MethodPost, "/balda/api/v1/games", nil)
	req.Header.Set("X-API-Key", testAPIToken)
	req.Header.Set("X-API-Session", p1Sid)
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var createResp baldaapi.CreateGameResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	gameID := createResp.Game.Value.ID.Value.String()
	p1GameToken := createResp.GameToken.Value

	// player 1 subscribes to lobby and game channel before player 2 joins
	lobbyCh := subscribeToChannel(t, wsURL, centrifugo.ChannelLobby, p1CfToken, p1LobbyToken)
	gameCh := subscribeToChannel(t, wsURL, centrifugo.ChannelGame(gameID), p1CfToken, p1GameToken)

	// player 2 joins
	w = httptest.NewRecorder()
	req = newReq(t, http.MethodPost, "/balda/api/v1/games/"+gameID+"/join", nil)
	req.Header.Set("X-API-Key", testAPIToken)
	req.Header.Set("X-API-Session", p2Sid)
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// check EvGameStarted on lobby
	select {
	case data := <-lobbyCh:
		var ev centrifugo.EvGameStarted
		require.NoError(t, json.Unmarshal(data, &ev))
		assert.Equal(t, "game_started", ev.Type)
		assert.Equal(t, "in_progress", ev.Status)
		assert.Equal(t, gameID, ev.GameID)
		assert.Len(t, ev.PlayerIDs, 2)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for EvGameStarted on lobby")
	}

	// check EvGameStarted on game channel
	select {
	case data := <-gameCh:
		var ev centrifugo.EvGameStarted
		require.NoError(t, json.Unmarshal(data, &ev))
		assert.Equal(t, "game_started", ev.Type)
		assert.Equal(t, gameID, ev.GameID)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for EvGameStarted on game channel")
	}
}

func signupPlayer(t *testing.T, srv http.Handler, email string) (sid, cfToken, lobbyToken string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"firstname": "Test", "lastname": "Player",
		"email": email, "password": "pass",
	})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, newReq(t, http.MethodPost, "/balda/api/v1/signup", body))
	require.Equal(t, http.StatusOK, w.Code)

	var resp baldaapi.SignupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.User.Value.Sid.Value, resp.CentrifugoToken.Value, resp.LobbyToken.Value
}

func newReq(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	var b *bytes.Reader
	if body != nil {
		b = bytes.NewReader(body)
	} else {
		b = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, path, b)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
