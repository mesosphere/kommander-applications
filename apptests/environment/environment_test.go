package environment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvision(t *testing.T) {
	env := Env{}
	err := env.Provision(context.Background())
	assert.NoError(t, err)
}
