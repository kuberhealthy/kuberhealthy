package health_test

import (
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/health"
	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	s := health.NewState()
	assert.True(t, s.OK)
}

func TestAddError(t *testing.T) {
	s := health.NewState()
	s.AddError("my error message")
	s.AddError("my another error message")

	assert.Contains(t, s.Errors, "my error message")
	assert.Contains(t, s.Errors, "my another error message")
}
