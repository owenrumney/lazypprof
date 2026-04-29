package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderFlags(t *testing.T) {
	var flags headerFlags
	require.NoError(t, flags.Set("Authorization: Bearer token"))
	require.NoError(t, flags.Set("X-Test: one"))
	require.NoError(t, flags.Set("X-Test: two"))

	headers := flags.Header()
	assert.Equal(t, "Bearer token", headers.Get("Authorization"))
	assert.Equal(t, http.Header{"Authorization": {"Bearer token"}, "X-Test": {"one", "two"}}, headers)
}

func TestHeaderFlagsRejectMalformed(t *testing.T) {
	var flags headerFlags
	assert.Error(t, flags.Set("Authorization"))
	assert.Error(t, flags.Set(": value"))
}
