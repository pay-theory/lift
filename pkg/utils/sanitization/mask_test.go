package sanitization

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskFirstLast4(t *testing.T) {
	require.Equal(t, "(empty)", MaskFirstLast4(""))
	require.Equal(t, "***masked***", MaskFirstLast4("12345678"))
	require.Equal(t, "1234***cdef", MaskFirstLast4("1234567890abcdef"))
}

func TestMaskFirstLast(t *testing.T) {
	require.Equal(t, "***masked***", MaskFirstLast("abcdef", 3, 3))
	require.Equal(t, "***masked***", MaskFirstLast("abcdef", -1, 2))
	require.Equal(t, "ab***ef", MaskFirstLast("abcdef", 2, 2))
}

func TestMaskBINLast4(t *testing.T) {
	require.Equal(t, "(empty)", MaskBINLast4(""))
	require.Equal(t, "***masked***", MaskBINLast4("1234567890"))
	require.Equal(t, "424242******4242", MaskBINLast4("4242424242424242"))
	require.Equal(t, "424242******4242", MaskBINLast4("4242-4242-4242-4242"))
}
