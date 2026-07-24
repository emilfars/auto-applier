package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// CardSelectors names the CSS classes used to extract a job card from a listing
// page. Keeping selectors as data (not code) means adapting to a portal's DOM
// change is a config edit, in line with the versioned-fill-map philosophy.
type CardSelectors struct {
	Card       string // container class for one listing
	Title      string
	Company    string
	Location   string
	Salary     string
	Employment string
	URL        string // class on the <a> whose href is the listing URL
}

// HTMLSource is a Tier 2 scraper for a public (no-login) HTML job board. It
// fetches a listing page and extracts cards using CardSelectors. Login-walled
// pages are out of scope by policy and must never be configured here.
type HTMLSource struct {
	id      string
	pageURL string
	client  *http.Client
	sel     CardSelectors
}

// NewHTMLSource builds a Tier 2 HTML source. A nil client uses a default with a
// timeout.
func NewHTMLSource(id, pageURL string, sel CardSelectors, client *http.Client) *HTMLSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &HTMLSource{id: id, pageURL: pageURL, client: client, sel: sel}
}

func (h *HTMLSource) ID() string { return h.id }

// Tier is always Tier 2 for an HTML board scraper.
func (h *HTMLSource) Tier() Tier { return Tier2 }

// Fetch downloads the listing page and returns one RawJob per card.
func (h *HTMLSource) Fetch(ctx context.Context) ([]RawJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ingest: build request for %s: %w", h.id, err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: fetch %s: %w", h.id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s returned status %d", h.id, resp.StatusCode)
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ingest: parse %s: %w", h.id, err)
	}

	base, _ := url.Parse(h.pageURL)
	var jobs []RawJob
	for _, card := range nodesWithClass(doc, h.sel.Card) {
		jobs = append(jobs, h.extract(card, base))
	}
	return jobs, nil
}

func (h *HTMLSource) extract(card *html.Node, base *url.URL) RawJob {
	return RawJob{
		Source:         h.id,
		SourceURL:      h.resolveURL(card, base),
		Title:          textOfClass(card, h.sel.Title),
		Company:        textOfClass(card, h.sel.Company),
		Location:       textOfClass(card, h.sel.Location),
		SalaryText:     textOfClass(card, h.sel.Salary),
		EmploymentType: textOfClass(card, h.sel.Employment),
	}
}

func (h *HTMLSource) resolveURL(card *html.Node, base *url.URL) string {
	a := firstNodeWithClass(card, h.sel.URL)
	if a == nil {
		return h.pageURL
	}
	href := attr(a, "href")
	if href == "" {
		return h.pageURL
	}
	if base != nil {
		if ref, err := url.Parse(href); err == nil {
			return base.ResolveReference(ref).String()
		}
	}
	return href
}

// --- small HTML helpers (stdlib x/net/html traversal) ---

func hasClass(n *html.Node, class string) bool {
	if class == "" || n.Type != html.ElementNode {
		return false
	}
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == class {
			return true
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodesWithClass(root *html.Node, class string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if hasClass(n, class) {
			out = append(out, n)
			return // do not descend into nested cards
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func firstNodeWithClass(root *html.Node, class string) *html.Node {
	if class == "" {
		return nil
	}
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if hasClass(n, class) {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func textOfClass(root *html.Node, class string) string {
	n := firstNodeWithClass(root, class)
	if n == nil {
		return ""
	}
	return strings.TrimSpace(textContent(n))
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}
