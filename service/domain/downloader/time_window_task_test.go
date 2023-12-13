package downloader_test

import (
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
	"github.com/stretchr/testify/require"
)

func TestTimeWindowTask_ReportingErrorsAfterTaskIsConsideredToBeDoneShouldBeIgnored(t *testing.T) {
	ctx := fixtures.TestContext(t)

	task, err := downloader.NewTimeWindowTask(ctx, nil, nil, nil, fixtures.SomeTimeWindow())
	require.NoError(t, err)

	task.OnReceivedEOSE()
	task.OnError(fixtures.SomeError())

	ok := task.CheckIfDoneAndEnd()
	require.True(t, ok)
}
