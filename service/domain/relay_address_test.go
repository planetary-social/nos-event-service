package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRelayAddress(t *testing.T) {
	testCases := []struct {
		Input  string
		Output string
	}{
		{
			Input:  "wss://example.com",
			Output: "wss://example.com",
		},
		{
			Input:  "ws://example.com",
			Output: "ws://example.com",
		},

		{
			Input:  " wss://example.com",
			Output: "wss://example.com",
		},
		{
			Input:  "wss://example.com ",
			Output: "wss://example.com",
		},
		{
			Input:  " wss://example.com ",
			Output: "wss://example.com",
		},

		{
			Input:  "wss://example.com/",
			Output: "wss://example.com",
		},
		{
			Input:  "wss://example.com/ ",
			Output: "wss://example.com",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Input, func(t *testing.T) {
			result, err := NewRelayAddress(testCase.Input)
			require.NoError(t, err)
			require.Equal(t, testCase.Output, result.String())
		})
	}
}
