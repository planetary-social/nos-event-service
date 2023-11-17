package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	s := NewEmptySet[int]()

	require.False(t, s.Contains(0))
	require.Equal(t, 0, s.Len())
	require.Len(t, s.List(), 0)

	s.Put(0)

	require.True(t, s.Contains(0))
	require.Equal(t, 1, s.Len())
	require.Equal(t, []int{0}, s.List())
}

func TestSet_Delete(t *testing.T) {
	s := NewEmptySet[int]()

	require.False(t, s.Contains(0))

	s.Put(0)

	require.True(t, s.Contains(0))

	s.Delete(0)

	require.False(t, s.Contains(0))
}

func TestSet_Clear(t *testing.T) {
	s := NewEmptySet[int]()

	require.False(t, s.Contains(0))

	s.Put(0)

	require.True(t, s.Contains(0))

	s.Clear()

	require.False(t, s.Contains(0))
}

func TestSet_Equal(t *testing.T) {
	require.True(t, NewSetVariadic(1, 2, 3).Equal(NewSetVariadic(1, 2, 3)))
	require.False(t, NewSetVariadic(1, 2).Equal(NewSetVariadic(1, 2, 3)))
	require.False(t, NewSetVariadic(1, 2, 3).Equal(NewSetVariadic(1, 2)))
	require.False(t, NewSetVariadic(1, 2, 4).Equal(NewSetVariadic(1, 2, 3)))
	require.False(t, NewSetVariadic(1, 2, 3).Equal(NewSetVariadic(1, 2, 4)))
	require.True(t, NewEmptySet[int]().Equal(NewEmptySet[int]()))
	require.False(t, NewSetVariadic(1, 2, 3).Equal(NewEmptySet[int]()))
	require.False(t, NewEmptySet[int]().Equal(NewSetVariadic(1, 2, 4)))
}
