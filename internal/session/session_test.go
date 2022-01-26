package session

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionService(t *testing.T) {
	svc := NewService(Config{Expiration: defaultExpiration})
	u := &User{
		Sid: "test_sid",
		UID: 1000,
	}
	err := svc.Save(u)
	assert.NoError(t, err)

	req := &http.Request{}
	u, err = svc.Get(req)
	assert.EqualError(t, err, ErrEmptySessionID.Error())

	req = &http.Request{Header: make(map[string][]string)}
	req.Header.Add("X-API-Session", "there_is_no_such_sid")
	u, err = svc.Get(req)
	assert.EqualError(t, err, ErrNotFound.Error())

	req = &http.Request{Header: make(map[string][]string)}
	req.Header.Add("X-API-Session", "test_sid")
	u, err = svc.Get(req)
	assert.NoError(t, err)
	assert.NotNil(t, u)
}
