package fetcher

import (
	"context"
	"log/slog"
)

func (f *Fetcher) initWorkers(ctx context.Context) {
	for i := 0; i < f.workers; i++ {
		f.wg.Add(1)
		go f.workerLoop(ctx)
	}
}

func (f *Fetcher) workerLoop(ctx context.Context) {
	defer f.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-f.eventPool:
			if err := f.processEvent(event); err != nil {
				slog.Error("failed to process event", "error", err)
			}
		}
	}
}
