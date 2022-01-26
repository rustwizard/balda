package session

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionService(t *testing.T) {
	svc := NewService(Config{Expiration: defaultExpiration})
	err := svc.Save(&User{
		Sid: "test_sid",
		UID: 1000,
	})
	assert.NoError(t, err)

	req := &http.Request{Header: make(map[string][]string)}
	req.Header.Add("X-API-Session", "test_sid")
	u, err := svc.Get(req)
	assert.NoError(t, err)
	assert.NotNil(t, u)
}
