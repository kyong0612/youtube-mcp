//go:build integration

// Package integration_test holds opt-in, network-dependent smoke tests for the
// YouTube MCP server.
//
// The DETERMINISTIC integration tests live in internal/youtube (white-box,
// fixture-driven) because they need to swap the HTTP transport. This file holds
// only the LIVE canary, which is skipped unless YT_LIVE_SMOKE is set: YouTube
// rate-limits/blocks CI IP ranges, so running it in the gating pipeline would
// make CI flaky.
package integration_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/kyong0612/youtube-mcp/internal/cache"
	"github.com/kyong0612/youtube-mcp/internal/config"
	"github.com/kyong0612/youtube-mcp/internal/youtube"
)

// TestLiveCanary_GetTranscript fetches a stable, well-known public video over the
// real network and asserts a non-empty transcript. It is OPT-IN: set
// YT_LIVE_SMOKE=1 to run. It is intentionally NOT part of the gating CI job.
func TestLiveCanary_GetTranscript(t *testing.T) {
	if os.Getenv("YT_LIVE_SMOKE") == "" {
		t.Skip("live canary skipped; set YT_LIVE_SMOKE=1 to hit the real YouTube network")
	}

	cfg := config.DefaultConfig().YouTube
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	memCache := cache.NewMemoryCache(100, 64, time.Minute)
	t.Cleanup(func() { _ = memCache.Close() })

	svc := youtube.NewService(cfg, memCache, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// "Me at the zoo" - the first YouTube video; stable and unlikely to disappear.
	const videoID = "jNQXAC9IVRw"

	resp, err := svc.GetTranscript(ctx, videoID, []string{"en"}, false)
	if err != nil {
		t.Fatalf("live GetTranscript(%s) error: %v", videoID, err)
	}
	if len(resp.Transcript) == 0 {
		t.Fatalf("live GetTranscript(%s) returned no transcript segments", videoID)
	}
	if resp.FormattedText == "" {
		t.Errorf("live GetTranscript(%s) returned empty formatted text", videoID)
	}

	t.Logf("live canary OK: %d segments, %d chars", len(resp.Transcript), resp.CharCount)
}
