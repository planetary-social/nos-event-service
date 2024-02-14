package domain

import (
	"testing"

	"github.com/boreq/errors"
	"github.com/stretchr/testify/require"
)

func TestRelayAddress(t *testing.T) {
	testCases := []struct {
		Input         string
		Output        string
		ExpectedError error
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
		{
			Input:  "wss://EXAMPLE.com/FooBar ",
			Output: "wss://example.com/FooBar",
		},
		{
			Input:  "wss://example1.com/ wss://example2.com",
			Output: "wss://example1.com/%20wss://example2.com",
		},
		{
			Input:         "wss:// wss://example.com",
			ExpectedError: errors.New("url parse error: parse \"wss:// wss://example.com\": invalid character \" \" in host name"),
		},
		{
			Input:  "wss://https://example.com",
			Output: "wss://https://example.com",
		},
		{
			Input:  "wss://example.",
			Output: "wss://example.",
		},
		{
			Input:         "https://example.com",
			ExpectedError: errors.New("invalid protocol"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Input, func(t *testing.T) {
			result, err := NewRelayAddress(testCase.Input)
			if testCase.ExpectedError != nil {
				require.EqualError(t, err, testCase.ExpectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.Output, result.String())
			}
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
			Input:  "ws://127.0.0.1:1234",
			Result: true,
		},
		{
			Input:  "ws://127.0.0.1:1234/path",
			Result: true,
		},
		{
			Input:  "ws://192.168.0.10",
			Result: true,
		},
		{
			Input:  "ws://192.168.0.10:1234",
			Result: true,
		},
		{
			Input:  "ws://192.168.0.10:1234/path",
			Result: true,
		},
		{
			Input:  "ws://1.2.3.4",
			Result: false,
		},
		{
			Input:  "ws://1.2.3.4:1234",
			Result: false,
		},
		{
			Input:  "ws://1.2.3.4:1234/path",
			Result: false,
		},
		{
			Input:  "ws://example.com",
			Result: false,
		},
		{
			Input:  "ws://example.com:1234",
			Result: false,
		},
		{
			Input:  "ws://example.com:1234/path",
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

func TestRelayAddress_Compare(t *testing.T) {
	a := MustNewRelayAddress("wss://example1.com")
	b := MustNewRelayAddress("wss://example1.com")
	c := MustNewRelayAddress("wss://example2.com")
	require.True(t, a == b)
	require.True(t, a != c)
}
