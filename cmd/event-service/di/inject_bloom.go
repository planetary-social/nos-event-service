package di

import (
	"time"

	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/service/domain/bloom"
)

var bloomSet = wire.NewSet(
	provideBloomFilter,
)

func provideBloomFilter() *bloom.EventFilter {
	config := bloom.FilterConfig{
		ExpectedItems:     1_000_000, // Expect 1M events per rotation
		FalsePositiveRate: 0.01,      // 1% false positive rate
		RotationInterval:  time.Hour, // Rotate every hour
	}
	return bloom.NewEventFilter(config)
}
