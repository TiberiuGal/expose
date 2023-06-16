package expose

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHostname(t *testing.T) {
	assert.Equal(t, "tibi", getHostname("tibi"))
	assert.Equal(t, "tibi", getHostname("tibi:80"))
	assert.Equal(t, "tibi", getHostname("tibi.dev:80"))
	assert.Equal(t, "tibi", getHostname("tibi.dev"))
	assert.Equal(t, "tibi", getHostname("tibi-proxy.dev"))
}
