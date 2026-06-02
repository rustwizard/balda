package rnd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func FuzzInt(f *testing.F) {
	f.Fuzz(func(t *testing.T, a int) {
		if a <= 0 {
			t.Skip()
		}
		val, err := Int(a)
		assert.NoError(t, err)
		assert.Less(t, val, a)
	})
}
