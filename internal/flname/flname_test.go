package flname

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenFLName(t *testing.T) {
	firstName, secondName := GenFLName()
	assert.NotEmpty(t, firstName, secondName)
	t.Logf("firstName: %s, secondName: %s", firstName, secondName)
}
