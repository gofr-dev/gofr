package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleDump = `# GoFr — full content dump

> Concatenated plaintext of every public docs page.

---

# Quick Start

## https://gofr.dev/docs/quick-start/introduction

# Hello, GoFr

GoFr is an opinionated Go framework for microservices.

---

## https://gofr.dev/docs/advanced-guide/using-publisher-subscriber

# Publisher Subscriber

GoFr supports Kafka, NATS and MQTT for pub sub. Kafka is the default.

---

## https://gofr.dev/faq

Frequently asked questions about GoFr and Kafka.

---
`

func TestParseCorpus(t *testing.T) {
	pages := parseCorpus(sampleDump)

	require.Len(t, pages, 3)

	assert.Equal(t, "/docs/quick-start/introduction", pages[0].Route)
	assert.Equal(t, "Hello, GoFr", pages[0].Title)
	assert.Contains(t, pages[0].Content, "opinionated Go framework")
	assert.Equal(t, "https://gofr.dev/docs/quick-start/introduction", pages[0].URL())

	// A page whose body opens with prose has no h1 to borrow, so the
	// route stands in as its title.
	assert.Equal(t, "/faq", pages[2].Title)
}

func TestParseCorpusEmpty(t *testing.T) {
	assert.Empty(t, parseCorpus(""))
	assert.Empty(t, parseCorpus("# Just a heading\n\nNo page markers here.\n"))
}

func TestPageHeading(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantRoute string
		wantOK    bool
	}{
		{"page marker", "## https://gofr.dev/docs/x", "/docs/x", true},
		{"section heading", "# Quick Start", "", false},
		{"other site", "## https://example.com/docs/x", "", false},
		{"no route", "## https://gofr.dev", "", false},
		{"prose", "some text", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route, ok := pageHeading(tc.line)

			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantRoute, route)
		})
	}
}

func TestSearch(t *testing.T) {
	pages := parseCorpus(sampleDump)

	t.Run("ranks title matches above body mentions", func(t *testing.T) {
		results := search(pages, "publisher subscriber", 10)

		require.NotEmpty(t, results)
		assert.Equal(t, "/docs/advanced-guide/using-publisher-subscriber", results[0].Page.Route)
	})

	t.Run("requires every term to match", func(t *testing.T) {
		assert.Empty(t, search(pages, "kafka nonexistentterm", 10))
	})

	t.Run("honors the limit", func(t *testing.T) {
		assert.Len(t, search(pages, "gofr", 1), 1)
	})

	t.Run("empty query returns nothing", func(t *testing.T) {
		assert.Empty(t, search(pages, "   ", 10))
	})

	t.Run("is case insensitive", func(t *testing.T) {
		assert.NotEmpty(t, search(pages, "KAFKA", 10))
	})
}

func TestExcerpt(t *testing.T) {
	t.Run("windows around the first match", func(t *testing.T) {
		content := strings.Repeat("padding ", 100) + "NEEDLE" + strings.Repeat(" tail", 100)

		got := excerpt(content, "needle")

		assert.Contains(t, got, "NEEDLE")
		assert.LessOrEqual(t, len([]rune(got)), excerptRunes+1)
	})

	t.Run("short content is returned whole", func(t *testing.T) {
		assert.Equal(t, "short body", excerpt("short body", "body"))
	})

	t.Run("does not split multi-byte runes", func(t *testing.T) {
		content := strings.Repeat("日本語テキスト", 200)

		assert.LessOrEqual(t, len([]rune(excerpt(content, "日本"))), excerptRunes+1)
	})
}

func TestFindPage(t *testing.T) {
	pages := parseCorpus(sampleDump)

	for _, path := range []string{
		"/docs/quick-start/introduction",
		"docs/quick-start/introduction",
		"/docs/quick-start/introduction.md",
		"/docs/quick-start/introduction/",
		"  /docs/quick-start/introduction  ",
	} {
		t.Run(path, func(t *testing.T) {
			p, ok := findPage(pages, path)

			require.True(t, ok)
			assert.Equal(t, "/docs/quick-start/introduction", p.Route)
		})
	}

	t.Run("unknown path", func(t *testing.T) {
		_, ok := findPage(pages, "/docs/nope")

		assert.False(t, ok)
	})

	t.Run("empty path", func(t *testing.T) {
		_, ok := findPage(pages, "")

		assert.False(t, ok)
	})
}

func TestSections(t *testing.T) {
	counts := sections(parseCorpus(sampleDump))

	assert.Equal(t, 1, counts["docs/quick-start"])
	assert.Equal(t, 1, counts["docs/advanced-guide"])
	assert.Equal(t, 1, counts["faq"])
}

func TestCorpusLoad(t *testing.T) {
	t.Run("fetches and parses once", func(t *testing.T) {
		calls := 0

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++

			_, _ = w.Write([]byte(sampleDump))
		}))
		defer srv.Close()

		c := newCorpus(srv.Client(), srv.URL)

		first, err := c.load(context.Background())
		require.NoError(t, err)
		require.Len(t, first, 3)

		second, err := c.load(context.Background())
		require.NoError(t, err)

		assert.Len(t, second, 3)
		assert.Equal(t, 1, calls, "corpus should be fetched at most once")
	})

	t.Run("non-200 is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		defer srv.Close()

		_, err := newCorpus(srv.Client(), srv.URL).load(context.Background())

		require.ErrorIs(t, err, errUnexpectedStatus)
	})

	t.Run("empty body is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("nothing parseable"))
		}))
		defer srv.Close()

		_, err := newCorpus(srv.Client(), srv.URL).load(context.Background())

		require.ErrorIs(t, err, errNoPages)
	})

	t.Run("recovers after a transient failure", func(t *testing.T) {
		// The regression this guards: with sync.Once, the first failure
		// was cached forever and every later call in that editor session
		// returned the same stale error.
		fail := true

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if fail {
				w.WriteHeader(http.StatusBadGateway)

				return
			}

			_, _ = w.Write([]byte(sampleDump))
		}))
		defer srv.Close()

		c := newCorpus(srv.Client(), srv.URL)

		_, err := c.load(context.Background())
		require.ErrorIs(t, err, errUnexpectedStatus)

		fail = false

		pages, err := c.load(context.Background())
		require.NoError(t, err)
		assert.Len(t, pages, 3)
	})

	t.Run("unreachable host is an error", func(t *testing.T) {
		_, err := newCorpus(http.DefaultClient, "http://127.0.0.1:1/llms-full.txt").
			load(context.Background())

		require.Error(t, err)
	})

	t.Run("invalid url is an error", func(t *testing.T) {
		_, err := newCorpus(http.DefaultClient, "://bad").load(context.Background())

		require.Error(t, err)
	})
}
