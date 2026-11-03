package httpserver

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	htmlnode "golang.org/x/net/html"
)

// Email HTML is shown in an iframe sandboxed without allow-scripts (popups
// only, so links can open) and a CSP that only allows inline styles and data:
// images. The policy keeps the presentational markup mail layouts depend on
// (inline CSS, table attributes, classes) and removes scripts, event handlers,
// forms and embedded content. <style> blocks are dropped; remote image URLs are
// dropped while their alt text and box stay. Links keep only http, https and
// mailto URLs and open in a new tab without opener or referrer.
var workspaceHTMLPolicy = newWorkspaceHTMLPolicy()

// AllowDataURIImages registers data: for every URL attribute, so links would
// accept data:image/… too; bluemonday cannot scope a scheme to <img>, so the
// href is removed from its output, where attribute values are escaped and
// double-quoted.
var workspaceDataLink = regexp.MustCompile(`(<(?:a|area)\b[^>]*?)\s+href="data:[^"]*"`)

var workspaceHTMLColor = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|[0-9a-fA-F]{6}|[a-zA-Z]{3,20})$`)

var workspaceEmailStyles = []string{
	"background", "background-color", "border", "border-top", "border-right", "border-bottom", "border-left",
	"border-color", "border-style", "border-width", "border-radius", "border-collapse", "border-spacing",
	"color", "display", "font", "font-family", "font-size", "font-style", "font-weight", "height", "max-height",
	"min-height", "width", "max-width", "min-width", "letter-spacing", "line-height", "list-style-type",
	"margin", "margin-top", "margin-right", "margin-bottom", "margin-left", "opacity", "overflow",
	"padding", "padding-top", "padding-right", "padding-bottom", "padding-left", "table-layout",
	"text-align", "text-decoration", "text-indent", "text-transform", "vertical-align", "visibility",
	"white-space", "word-break", "word-wrap", "overflow-wrap",
}

func newWorkspaceHTMLPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowRelativeURLs(false)
	p.AllowURLSchemes("http", "https", "mailto")
	p.AddTargetBlankToFullyQualifiedLinks(true) // also adds rel="noopener"
	p.RequireNoReferrerOnLinks(true)
	p.RequireNoFollowOnLinks(true)
	p.AllowDataURIImages()
	p.RewriteSrc(func(u *url.URL) { // remote images keep alt and size but are never fetched
		if u.Scheme != "data" {
			*u = url.URL{}
		}
	})
	p.AllowStyling()
	p.AllowStyles(workspaceEmailStyles...).Globally()
	p.AllowElements("center", "header", "footer", "main", "nav")
	p.AllowAttrs("color").Matching(workspaceHTMLColor).OnElements("font")
	p.AllowAttrs("face").Matching(bluemonday.Paragraph).OnElements("font")
	p.AllowAttrs("size").Matching(regexp.MustCompile(`^[-+]?[1-7]$`)).OnElements("font")
	p.AllowAttrs("align").Matching(bluemonday.CellAlign).OnElements("table", "div", "p", "h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowAttrs("bgcolor").Matching(workspaceHTMLColor).OnElements("table", "thead", "tbody", "tfoot", "tr", "td", "th")
	p.AllowAttrs("cellpadding", "cellspacing", "border").Matching(bluemonday.Integer).OnElements("table")
	p.AllowAttrs("border").Matching(bluemonday.Integer).OnElements("img")
	return p
}

func sanitizeWorkspaceHTML(src string) string {
	return workspaceDataLink.ReplaceAllString(workspaceHTMLPolicy.Sanitize(src), "$1")
}

type workspaceLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Host  string `json:"host"`
}

func workspaceMailLinks(src string) []workspaceLink {
	result := []workspaceLink{}
	seen := map[string]bool{}
	tokenizer := htmlnode.NewTokenizer(strings.NewReader(src))
	tokenizer.SetMaxBuf((2 << 20) + 1)
	for len(result) < 30 {
		kind := tokenizer.Next()
		if kind == htmlnode.ErrorToken {
			break
		}
		if kind != htmlnode.StartTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "a" {
			continue
		}
		for _, attr := range token.Attr {
			if attr.Key != "href" {
				continue
			}
			value := strings.TrimSpace(attr.Val)
			if len(value) > 8192 || strings.ContainsFunc(value, unicode.IsControl) {
				continue
			}
			u, err := url.Parse(value)
			if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || seen[value] {
				continue
			}
			seen[value] = true
			result = append(result, workspaceLink{Label: "邮件链接", URL: value, Host: u.Hostname()})
		}
	}
	return result
}
