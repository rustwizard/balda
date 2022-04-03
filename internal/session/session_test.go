package session

import (
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

	u, err = svc.Get(1000)
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), u.UID)

	u, err = svc.Get(1001)
	assert.EqualError(t, ErrNotFound, err.Error())
	assert.Nil(t, u)

}
