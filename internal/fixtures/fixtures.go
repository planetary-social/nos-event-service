package fixtures

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func SomePublicKey() domain.PublicKey {
	p, _ := SomeKeyPair()
	return p
}

func SomeKeyPair() (publicKey domain.PublicKey, secretKeyHex string) {
	hex := somePrivateKeyHex()

	p, err := nostr.GetPublicKey(hex)
	if err != nil {
		panic(err)
	}
	v, err := domain.NewPublicKeyFromHex(p)
	if err != nil {
		panic(err)
	}
	return v, hex
}

func SomeError() error {
	return fmt.Errorf("some error: %d", rand.Int())
}

func SomeEvent() domain.Event {
	libevent := nostr.Event{
		Kind: internal.RandomElement([]domain.EventKind{domain.EventKindContacts, domain.EventKindNote, domain.EventKindMetadata}).Int(),
		Tags: []nostr.Tag{
			{SomeString(), SomeString()},
		},
		Content: SomeString(),
	}

	_, sk := SomeKeyPair()
	err := libevent.Sign(sk)
	if err != nil {
		panic(err)
	}

	event, err := domain.NewEvent(libevent)
	if err != nil {
		panic(err)
	}

	return event
}

func SomeEventWithAuthor(sk string) domain.Event {
	libevent := nostr.Event{
		Kind: internal.RandomElement([]domain.EventKind{domain.EventKindContacts, domain.EventKindNote, domain.EventKindMetadata}).Int(),
		Tags: []nostr.Tag{
			{SomeString(), SomeString()},
		},
		Content: SomeString(),
	}

	err := libevent.Sign(sk)
	if err != nil {
		panic(err)
	}

	event, err := domain.NewEvent(libevent)
	if err != nil {
		panic(err)
	}

	return event
}

func Event(kind domain.EventKind, tags []domain.EventTag, content string) domain.Event {
	libevent := nostr.Event{
		Kind:    kind.Int(),
		Content: content,
	}

	for _, tag := range tags {
		libevent.Tags = append(libevent.Tags, nostr.Tag{tag.Name().String(), tag.FirstValue()})
	}

	_, sk := SomeKeyPair()
	err := libevent.Sign(sk)
	if err != nil {
		panic(err)
	}

	event, err := domain.NewEvent(libevent)
	if err != nil {
		panic(err)
	}

	return event
}

func SomeFile(t testing.TB) string {
	file, err := os.CreateTemp("", "nos-events-test")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		err := os.Remove(file.Name())
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(cleanup)

	return file.Name()
}

func SomeEventID() domain.EventId {
	return domain.MustNewEventId(SomeHexBytesOfLen(32))
}

func TestContext(t testing.TB) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func TestLogger(t testing.TB) logging.Logger {
	return logging.NewSystemLogger(logging.NewTestingLoggingSystem(t), "test")
}

func SomeString() string {
	return randSeq(10)
}

func SomeHexBytesOfLen(l int) string {
	b := make([]byte, l)
	n, err := cryptorand.Read(b)
	if n != len(b) {
		panic("short read")
	}
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func SomeBytesOfLen(l int) []byte {
	b := make([]byte, l)
	n, err := cryptorand.Read(b)
	if n != len(b) {
		panic("short read")
	}
	if err != nil {
		panic(err)
	}
	return b
}

func SomeRelayAddress() domain.RelayAddress {
	protocol := internal.RandomElement([]string{"ws", "wss"})
	address := fmt.Sprintf("%s://%s", protocol, SomeString())

	v, err := domain.NewRelayAddress(address)
	if err != nil {
		panic(err)
	}
	return v
}

func SomeEventKind() domain.EventKind {
	return internal.RandomElement([]domain.EventKind{domain.EventKindNote, domain.EventKindContacts})
}

func SomeMaybeRelayAddress() domain.MaybeRelayAddress {
	return domain.NewMaybeRelayAddress(SomeString())
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func somePrivateKeyHex() string {
	return nostr.GeneratePrivateKey()
}

func RequireEqualEventSlices(tb testing.TB, a, b []domain.Event) {
	require.Equal(tb, len(a), len(b))
	for i := 0; i < len(a); i++ {
		require.Equal(tb, a[i].Id(), b[i].Id())
		require.Equal(tb, a[i].Raw(), b[i].Raw())
	}
}
