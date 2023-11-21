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

func TestRelayAddress_IsLocal(t *testing.T) {
	testCases := []struct {
		Input  string
		Result bool
	}{
		{
			Input:  "ws://127.0.0.1",
			Result: true,
		},
		{
			Input:  "ws://192.168.0.10",
			Result: true,
		},
		{
			Input:  "ws://1.2.3.4",
			Result: false,
		},
		{
			Input:  "ws://example.com",
			Result: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Input, func(t *testing.T) {
			address := MustNewRelayAddress(testCase.Input)
			require.Equal(t, testCase.Result, address.IsLoopbackOrPrivate())
		})
	}
}
