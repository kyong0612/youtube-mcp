//go:build integration

package youtube

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyong0612/youtube-mcp/internal/config"
	"github.com/kyong0612/youtube-mcp/internal/models"
)

// fixtureVideoID is the 11-character video ID embedded in the watch-page fixture.
const fixtureVideoID = "TESTVIDEO01"

// fixtureTransport is a test-only http.RoundTripper that serves saved fixtures
// instead of reaching out to YouTube. Installing it on the Service's http.Client
// lets these tests drive the real, public fetch -> parse -> format pipeline
// (GetTranscript / FormatTranscript, including retry and status handling)
// fully offline and deterministically. No production code is modified; only the
// (unexported) transport field of the test's own Service is swapped.
//
// It routes by request path:
//   - "/watch"            -> the saved watch-page HTML (ytInitialPlayerResponse)
//   - any path containing "timedtext" -> the saved caption XML
type fixtureTransport struct {
	watchHTML   []byte
	captionXML  []byte
	watchHits   int
	captionHits int
}

func (ft *fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Request:    req,
	}

	switch {
	case req.URL.Path == "/watch":
		ft.watchHits++
		resp.StatusCode = http.StatusOK
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		resp.Body = io.NopCloser(bytes.NewReader(ft.watchHTML))
	case strings.Contains(req.URL.Path, "timedtext"):
		ft.captionHits++
		resp.StatusCode = http.StatusOK
		resp.Header.Set("Content-Type", "application/xml; charset=utf-8")
		resp.Body = io.NopCloser(bytes.NewReader(ft.captionXML))
	default:
		resp.StatusCode = http.StatusNotFound
		resp.Body = io.NopCloser(bytes.NewReader(nil))
	}

	return resp, nil
}

// newFixtureService builds a Service whose HTTP transport serves the given
// caption fixture (and the shared watch-page fixture), with a fresh in-memory
// mock cache. Rate limits are set generously so the test never blocks.
func newFixtureService(t *testing.T, captionFixture string) (*Service, *fixtureTransport) {
	t.Helper()

	cfg := config.YouTubeConfig{
		DefaultLanguages:   []string{"en"},
		RequestTimeout:     5 * time.Second,
		RetryAttempts:      2,
		RetryDelay:         time.Millisecond,
		RateLimitPerMinute: 600,
		RateLimitPerHour:   6000,
		MaxConcurrent:      4,
		UserAgent:          "youtube-mcp-integration-test",
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(cfg, newMockCache(), logger)

	ft := &fixtureTransport{
		watchHTML:  readFixture(t, "watch_page.html"),
		captionXML: readFixture(t, captionFixture),
	}
	svc.httpClient.Transport = ft

	return svc, ft
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", name, err)
	}
	return data
}

// TestIntegration_FetchParseFormatPipeline exercises the full public path:
// GetTranscript fetches the watch page over HTTP (intercepted), parses
// ytInitialPlayerResponse, selects the caption track, fetches the caption XML
// over HTTP (intercepted), parses it into segments, and produces the default
// formatted text. It asserts the actual fetch happened and the parsed/formatted
// output is exactly as expected.
func TestIntegration_FetchParseFormatPipeline(t *testing.T) {
	svc, ft := newFixtureService(t, "timedtext_transcript.xml")

	resp, err := svc.GetTranscript(context.Background(), fixtureVideoID, []string{"en"}, false)
	if err != nil {
		t.Fatalf("GetTranscript returned error: %v", err)
	}

	// The fetch path was actually exercised over the (intercepted) HTTP client.
	if ft.watchHits == 0 {
		t.Error("expected the watch page to be fetched through the HTTP client")
	}
	if ft.captionHits == 0 {
		t.Error("expected the caption track URL to be fetched through the HTTP client")
	}

	// Metadata parsed from the watch-page fixture.
	if resp.VideoID != fixtureVideoID {
		t.Errorf("VideoID = %q, want %q", resp.VideoID, fixtureVideoID)
	}
	if resp.Title != "Integration Test Video" {
		t.Errorf("Title = %q, want %q", resp.Title, "Integration Test Video")
	}
	if resp.Language != "en" {
		t.Errorf("Language = %q, want %q", resp.Language, "en")
	}
	if resp.Metadata.ChannelName != "Test Channel" {
		t.Errorf("Metadata.ChannelName = %q, want %q", resp.Metadata.ChannelName, "Test Channel")
	}
	if resp.Metadata.ViewCount != 12345 {
		t.Errorf("Metadata.ViewCount = %d, want 12345", resp.Metadata.ViewCount)
	}

	// Segments parsed from the caption XML fixture.
	wantTexts := []string{"Hello and welcome", "to the integration test", "of the YouTube MCP server"}
	if got := len(resp.Transcript); got != len(wantTexts) {
		t.Fatalf("segment count = %d, want %d", got, len(wantTexts))
	}
	for i, want := range wantTexts {
		if resp.Transcript[i].Text != want {
			t.Errorf("segment[%d].Text = %q, want %q", i, resp.Transcript[i].Text, want)
		}
	}
	if resp.Transcript[0].Start != 0 || resp.Transcript[0].End != 2.5 {
		t.Errorf("segment[0] timing = (%.3f, %.3f), want (0, 2.5)", resp.Transcript[0].Start, resp.Transcript[0].End)
	}
	if resp.DurationSeconds != 8.0 {
		t.Errorf("DurationSeconds = %.3f, want 8.0", resp.DurationSeconds)
	}

	// Default formatted text.
	wantPlain := "Hello and welcome to the integration test of the YouTube MCP server"
	if resp.FormattedText != wantPlain {
		t.Errorf("FormattedText = %q, want %q", resp.FormattedText, wantPlain)
	}
}

// TestIntegration_FormatTranscriptOutputs drives FormatTranscript end-to-end for
// the timestamped output formats and asserts the exact serialized result.
func TestIntegration_FormatTranscriptOutputs(t *testing.T) {
	cases := []struct {
		name       string
		formatType string
		want       string
	}{
		{
			name:       "plain_text",
			formatType: models.FormatTypePlainText,
			want:       "Hello and welcome to the integration test of the YouTube MCP server",
		},
		{
			name:       "srt",
			formatType: models.FormatTypeSRT,
			want: "1\n00:00:00,000 --> 00:00:02,500\nHello and welcome\n\n" +
				"2\n00:00:02,500 --> 00:00:05,500\nto the integration test\n\n" +
				"3\n00:00:05,500 --> 00:00:08,000\nof the YouTube MCP server",
		},
		{
			name:       "vtt",
			formatType: models.FormatTypeVTT,
			want: "WEBVTT\n\n" +
				"00:00:00.000 --> 00:00:02.500\nHello and welcome\n\n" +
				"00:00:02.500 --> 00:00:05.500\nto the integration test\n\n" +
				"00:00:05.500 --> 00:00:08.000\nof the YouTube MCP server",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newFixtureService(t, "timedtext_transcript.xml")

			resp, err := svc.FormatTranscript(context.Background(), fixtureVideoID, tc.formatType, false)
			if err != nil {
				t.Fatalf("FormatTranscript(%s) error: %v", tc.formatType, err)
			}
			if resp.FormattedText != tc.want {
				t.Errorf("FormatTranscript(%s) mismatch\n got: %q\nwant: %q", tc.formatType, resp.FormattedText, tc.want)
			}
		})
	}
}

// TestIntegration_TimedTextFormatPipeline exercises the same public path but with
// a caption response in YouTube's <timedtext> format, covering the second branch
// of parseTranscriptXML through the real fetch path.
func TestIntegration_TimedTextFormatPipeline(t *testing.T) {
	svc, ft := newFixtureService(t, "timedtext_v3.xml")

	resp, err := svc.GetTranscript(context.Background(), fixtureVideoID, []string{"en"}, false)
	if err != nil {
		t.Fatalf("GetTranscript returned error: %v", err)
	}
	if ft.captionHits == 0 {
		t.Error("expected the caption track URL to be fetched through the HTTP client")
	}

	wantTexts := []string{"Hello and welcome", "to the timedtext format"}
	if got := len(resp.Transcript); got != len(wantTexts) {
		t.Fatalf("segment count = %d, want %d", got, len(wantTexts))
	}
	for i, want := range wantTexts {
		if resp.Transcript[i].Text != want {
			t.Errorf("segment[%d].Text = %q, want %q", i, resp.Transcript[i].Text, want)
		}
	}

	wantPlain := "Hello and welcome to the timedtext format"
	if resp.FormattedText != wantPlain {
		t.Errorf("FormattedText = %q, want %q", resp.FormattedText, wantPlain)
	}
}
