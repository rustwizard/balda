package flname

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenFLName(t *testing.T) {
	for i := 0; i < 3; i++ {
		firstName, secondName := GenFLName()
		assert.NotEmpty(t, firstName, secondName)
		t.Logf("firstName: %s, secondName: %s", firstName, secondName)
	}
}
