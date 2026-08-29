// Package webhelp is a small, deliberately narrow addition: an
// OPTIONAL, best-effort live fetch of a doc chapter's raw markdown,
// to supplement (never replace) the embedded help text every command
// already prints unconditionally. Every property here follows from
// one constraint: -h/--help has been fully offline and deterministic
// since this project began, and that must not change just because
// this exists.
//
// This fetch is a direct, self-contained HTTP request from this
// compiled binary -- it does not depend on, or route through, any
// other tool's own URL-fetching policy.
//
// Why raw markdown, not the rendered HTML page: the HTML page is
// meant for a browser, wrapped in page chrome (nav, footer, CSS)
// that would need stripping to be useful printed to a terminal. The
// raw .md source is plain text already, exactly what a terminal
// wants, and requires no HTML parsing at all -- gorepoman's own
// GitHub Pages mirror serves both forms of every chapter side by
// side for exactly this reason.
//
// Why REPOMAN_NO_WEB_HELP exists: `repoman selftest`, the acceptance
// gate, calls -h on multiple commands as part of its own checks.
// Those calls must never depend on network reachability -- a
// selftest that could fail, hang, or simply run slower because of
// network conditions during a test run would compromise the one
// property selftest exists to guarantee. Every internal invocation
// this project's own test harness makes sets this variable; nothing
// else needs to.
package webhelp

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// baseURL is gorepoman's own GitHub Pages mirror -- built by CI from
// the same tagged release as the actual binaries, not a third-party
// or user-supplied host. Overridable via REPOMAN_WEBHELP_BASE_URL,
// primarily so this project's own selftest can verify the full
// fetch-and-print pipeline against a local, in-process test server --
// deterministic and network-independent -- rather than either
// skipping success-path coverage entirely or making the acceptance
// gate depend on real network reachability to get it.
func resolveBaseURL() string {
	if v := os.Getenv("REPOMAN_WEBHELP_BASE_URL"); v != "" {
		return v
	}
	return "https://ha1tch.github.io/gorepoman/docs/"
}

// browserUA is a realistic, current desktop-browser user agent. Not
// confirmed necessary against this specific host (direct testing
// found no user-agent-based difference in reachability here) -- set
// anyway since it costs nothing and may matter against hosts or
// configurations not tested.
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

// fetchTimeout is deliberately short: whoever is waiting on -h
// output is waiting on something that has always been instant.
// Long enough for a real, working connection; short enough that an
// unreachable or slow network doesn't make -h itself feel broken.
const fetchTimeout = 3 * time.Second

// FetchDoc attempts to fetch the raw markdown for the doc chapter
// named by docBase (e.g. "repoman-060-register-and-guards", no
// extension). Returns the content on a genuine success (network
// reachable, HTTP 200, a real non-empty body); returns "" on every
// other outcome -- timeout, DNS failure, any non-200 status, an
// empty body, or REPOMAN_NO_WEB_HELP being set. Callers print
// nothing extra when this returns "" -- there is no error to
// surface, since this was always an optional addition, not a
// requirement.
func FetchDoc(docBase string) string {
	if os.Getenv("REPOMAN_NO_WEB_HELP") != "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolveBaseURL()+docBase+".md", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", browserUA)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Cap the read: this is meant to be one doc chapter, not an
	// unbounded download -- 2MB is generous headroom over any real
	// chapter in this project while still bounding a pathological
	// response.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	return text
}

// PrintIfAvailable prints a clearly-delineated section containing
// the live-fetched chapter, if and only if the fetch succeeds.
// Prints nothing at all otherwise -- the caller's own embedded help
// text, printed before this is ever called, is unaffected either
// way.
func PrintIfAvailable(w io.Writer, docBase string) {
	content := FetchDoc(docBase)
	if content == "" {
		return
	}
	io.WriteString(w, "\n----------------------------------------\n")
	io.WriteString(w, "Full chapter, fetched live from the docs site:\n")
	io.WriteString(w, "----------------------------------------\n\n")
	io.WriteString(w, content)
	io.WriteString(w, "\n")
}
