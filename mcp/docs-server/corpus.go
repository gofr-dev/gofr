package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

var (
	errUnexpectedStatus = errors.New("unexpected status fetching documentation")
	errNoPages          = errors.New("no documentation pages could be parsed")
)

// page is one documentation page parsed out of llms-full.txt.
type page struct {
	Route   string
	Title   string
	Content string
}

// URL returns the page's canonical address on the site.
func (p page) URL() string { return siteURL + p.Route }

// corpus is the parsed documentation, fetched once and then reused.
//
// The fetch is deferred until the first tool call rather than done at
// startup: an MCP client launches the server eagerly when the editor
// opens, and a doc fetch that fails there would surface as "the server
// crashed" rather than "that one call could not reach the network".
//
// Only SUCCESS is latched. sync.Once would have cached a failure too, so
// a single flaky moment at the first tool call would leave every later
// call in that editor session returning the same stale error, curable
// only by restarting the server. Retrying costs one request after a
// failure and keeps the process usable.
type corpus struct {
	client *http.Client
	url    string
	mu     sync.Mutex
	pages  []page
}

func newCorpus(client *http.Client, url string) *corpus {
	return &corpus{client: client, url: url}
}

func (c *corpus) load(ctx context.Context) ([]page, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pages) > 0 {
		return c.pages, nil
	}

	pages, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}

	c.pages = pages

	return c.pages, nil
}

func (c *corpus) fetch(ctx context.Context) ([]page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", c.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s returned %d", errUnexpectedStatus, c.url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", c.url, err)
	}

	pages := parseCorpus(string(body))
	if len(pages) == 0 {
		return nil, fmt.Errorf("%w from %s", errNoPages, c.url)
	}

	return pages, nil
}

// parseCorpus splits llms-full.txt into pages.
//
// The dump marks each page with an `## <absolute URL>` heading and
// separates pages with a `---` rule. Section headings (`# Quick Start`)
// sit between groups of pages and are skipped — they carry no content of
// their own.
func parseCorpus(dump string) []page {
	var (
		pages   []page
		current *page
		body    strings.Builder
	)

	flush := func() {
		if current == nil {
			return
		}

		current.Content = strings.TrimSpace(body.String())
		if current.Content != "" {
			current.Title = firstHeading(current.Content, current.Route)
			pages = append(pages, *current)
		}

		current = nil

		body.Reset()
	}

	for _, line := range strings.Split(dump, "\n") {
		route, ok := pageHeading(line)
		if ok {
			flush()

			current = &page{Route: route}

			continue
		}

		if current != nil {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	flush()

	return pages
}

// pageHeading reports whether line opens a page, and for which route.
func pageHeading(line string) (string, bool) {
	const marker = "## " + siteURL

	if !strings.HasPrefix(line, marker) {
		return "", false
	}

	route := strings.TrimSpace(strings.TrimPrefix(line, "## "+siteURL))
	if route == "" || !strings.HasPrefix(route, "/") {
		return "", false
	}

	return route, true
}

// firstHeading returns the page's own h1, falling back to its route when
// the page opens with prose instead.
func firstHeading(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	return fallback
}

// searchResult is one hit, ordered by score descending.
type searchResult struct {
	Page  page
	Score int
}

// search ranks pages by how often the query terms appear, weighting the
// title and route above the body: a page *about* Kafka should outrank a
// page that mentions Kafka once in a list of supported brokers.
func search(pages []page, query string, limit int) []searchResult {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}

	var results []searchResult

	for _, p := range pages {
		title := strings.ToLower(p.Title + " " + p.Route)
		content := strings.ToLower(p.Content)

		score := 0
		matchedAll := true

		for _, term := range terms {
			hits := strings.Count(content, term) + 10*strings.Count(title, term)
			if hits == 0 {
				matchedAll = false
				break
			}

			score += hits
		}

		if matchedAll {
			results = append(results, searchResult{Page: p, Score: score})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}

		return results[i].Page.Route < results[j].Page.Route
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// excerpt returns a short, rune-safe window around the first match, so a
// client can judge relevance without fetching the whole page.
func excerpt(content, query string) string {
	runes := []rune(content)

	start := 0
	if idx := strings.Index(strings.ToLower(content), strings.ToLower(firstTerm(query))); idx > 0 {
		start = len([]rune(content[:idx]))
	}

	if start > excerptRunes/2 {
		start -= excerptRunes / 2
	} else {
		start = 0
	}

	end := start + excerptRunes
	if end > len(runes) {
		end = len(runes)
	}

	out := strings.TrimSpace(string(runes[start:end]))
	if end < len(runes) {
		out += "…"
	}

	return out
}

func firstTerm(query string) string {
	if fields := strings.Fields(query); len(fields) > 0 {
		return fields[0]
	}

	return query
}

// findPage resolves a site path to a page, tolerating a missing leading
// slash and a trailing `.md` — both are things an agent reasonably sends.
func findPage(pages []page, path string) (page, bool) {
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, ".md")

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}

	for _, p := range pages {
		if p.Route == path {
			return p, true
		}
	}

	return page{}, false
}

// sections groups routes by their top-level documentation section.
func sections(pages []page) map[string]int {
	counts := make(map[string]int)

	for _, p := range pages {
		parts := strings.Split(strings.Trim(p.Route, "/"), "/")

		name := parts[0]
		if name == "docs" && len(parts) > 1 {
			name = "docs/" + parts[1]
		}

		counts[name]++
	}

	return counts
}
