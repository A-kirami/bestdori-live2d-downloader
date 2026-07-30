package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveCharaColor(t *testing.T) {
	require.Equal(t, "#123456", resolveCharaColor(1, " #123456 "))
	require.Equal(t, "#DD33CC", resolveCharaColor(601, ""))
	require.Equal(t, fallbackCharaColor, resolveCharaColor(225, ""))
}
