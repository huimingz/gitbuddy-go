package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultChatConfig tests default chat configuration
func TestDefaultChatConfig(t *testing.T) {
	config := DefaultChatConfig()
	require.NotNil(t, config)

	assert.Equal(t, 10, config.MaxIterations)
	assert.Equal(t, 1000, config.MaxLinesPerRead)
	assert.True(t, config.EnableCompression)
	assert.Equal(t, 20, config.CompressionThreshold)
	assert.Equal(t, 10, config.CompressionKeepRecent)
	assert.Equal(t, 100, config.MaxHistory)
}
