package appscenarios

import (
	"context"
	"testing"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/stretchr/testify/assert"
)

func TestListExecute(t *testing.T) {
	env := &environment.Env{}
	ctx := context.Background()

	err := env.Provision(ctx)
	assert.NoError(t, err)
	defer env.Destroy(ctx)

	r := reloader{}
	err = r.Execute(ctx, env)
	assert.NoError(t, err)
}
