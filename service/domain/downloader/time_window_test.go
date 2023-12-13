package downloader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeWindow(t *testing.T) {
	start := time.Now()

	first, err := NewTimeWindow(start, time.Minute)
	require.NoError(t, err)

	require.Equal(t, start, first.Start())
	require.Equal(t, start.Add(time.Minute), first.End())

	second := first.Advance()
	require.Equal(t, start.Add(time.Minute), second.Start())
	require.Equal(t, start.Add(2*time.Minute), second.End())
	require.True(t, second.Start().Equal(first.End()))
}
