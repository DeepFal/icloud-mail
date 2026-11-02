package httpserver

import (
	"html"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	htmlnode "golang.org/x/net/html"
)

var workspaceHTMLAllowed = map[string]bool{"p": true, "br": true, "div": true, "span": true, "strong": true, "b": true, "em": true, "i": true, "u": true, "ul": true, "ol": true, "li": true, "blockquote": true, "pre": true, "code": true, "table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true, "th": true, "td": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true}
var workspaceHTMLDrop = map[string]bool{"script": true, "style": true, "iframe": true, "object": true, "embed": true, "form": true, "svg": true, "math": true, "template": true, "head": true, "input": true, "button": true, "link": true, "meta": true}

func sanitizeWorkspaceHTML(src string) string {
	doc, err := htmlnode.Parse(strings.NewReader(src))
	if err != nil {
		return ""
	}
	var out strings.Builder
	visited := 0
	var visit func(*htmlnode.Node, int)
	visit = func(n *htmlnode.Node, depth int) {
		visited++
		if depth > 128 || visited > 100000 {
			return
		}
		if n.Type == htmlnode.TextNode {
			out.WriteString(html.EscapeString(n.Data))
			return
		}
		if n.Type == htmlnode.CommentNode {
			return
		}
		tag := strings.ToLower(n.Data)
		if n.Type == htmlnode.ElementNode && workspaceHTMLDrop[tag] {
			return
		}
		if tag == "img" {
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "alt") {
					out.WriteString(html.EscapeString(a.Val))
				}
			}
			return
		}
		allowed := n.Type == htmlnode.ElementNode && workspaceHTMLAllowed[tag]
		if allowed {
			out.WriteByte('<')
			out.WriteString(tag)
			if tag == "td" || tag == "th" {
				for _, a := range n.Attr {
					key := strings.ToLower(a.Key)
					if key != "colspan" && key != "rowspan" {
						continue
					}
					v, e := strconv.Atoi(a.Val)
					if e == nil && v >= 1 && v <= 100 {
						out.WriteByte(' ')
						out.WriteString(key)
						out.WriteString(`="`)
						out.WriteString(strconv.Itoa(v))
						out.WriteByte('"')
					}
				}
			}
			out.WriteByte('>')
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child, depth+1)
		}
		if allowed && tag != "br" && tag != "hr" {
			out.WriteString("</")
			out.WriteString(tag)
			out.WriteByte('>')
		}
	}
	visit(doc, 0)
	return out.String()
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
