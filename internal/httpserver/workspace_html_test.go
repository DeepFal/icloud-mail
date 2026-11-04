package httpserver

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestSanitizeWorkspaceHTMLChecksEveryURLAttribute(t *testing.T) {
	const png = "data:image/png;base64,iVBORw0KGgo="
	const svg = "data:image/svg+xml;base64,PHN2Zy8+"
	for _, element := range []string{"a", "area"} {
		for _, test := range []struct {
			name string
			urls []string
			want []string
		}{
			{"duplicate data", []string{png, png}, nil},
			{"data first", []string{png, "https://example.test", png}, []string{"https://example.test"}},
			{"data last", []string{"http://example.test", png, png}, []string{"http://example.test"}},
			{"alternating", []string{png, "http://example.test", png, "https://example.test", png}, []string{"http://example.test", "https://example.test"}},
			{"safe", []string{"http://example.test", "https://example.test", "mailto:hi@example.test"}, []string{"http://example.test", "https://example.test", "mailto:hi@example.test"}},
			{"svg", []string{svg, svg}, nil},
			{"unsafe", []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", " java\t\nscript:alert(1)", "&#x20;java&#x73;cript&#58;alert(1)", "vbscript:x", "file:///etc/passwd"}, nil},
		} {
			t.Run(element+"/"+test.name, func(t *testing.T) {
				in := "<" + element
				for _, value := range test.urls {
					in += ` href="` + value + `"`
				}
				out := sanitizeWorkspaceHTML(in + ` alt="link">link</` + element + `>`)
				var got []string
				z := html.NewTokenizer(strings.NewReader(out))
				for kind := z.Next(); kind != html.ErrorToken; kind = z.Next() {
					if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
						continue
					}
					for _, attr := range z.Token().Attr {
						if attr.Key == "href" {
							u, err := url.Parse(attr.Val)
							if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "mailto") {
								t.Fatalf("unsafe href %q in %s", attr.Val, out)
							}
							got = append(got, attr.Val)
						}
					}
				}
				if strings.Join(got, "|") != strings.Join(test.want, "|") {
					t.Fatalf("hrefs = %v, want %v: %s", got, test.want, out)
				}
			})
		}
	}
	for _, mime := range []string{"png", "jpeg", "gif", "webp", "svg+xml"} {
		out := sanitizeWorkspaceHTML(`<img src="data:image/` + mime + `;base64,PHN2Zy8+" alt="inline">`)
		if strings.Contains(out, "src=") != (mime != "svg+xml") {
			t.Fatalf("unexpected image policy for %s: %s", mime, out)
		}
	}
	// Preserve the existing CID fallback: retain alt text without loading a resource.
	out := sanitizeWorkspaceHTML(`<img src="cid:missing" alt="cid fallback"><blockquote cite="` + png + `">quote</blockquote>`)
	if strings.Contains(out, "cid:missing") || strings.Contains(out, "data:") || !strings.Contains(out, `alt="cid fallback"`) {
		t.Fatalf("unexpected resource URL: %s", out)
	}
}

func TestSanitizeWorkspaceHTMLDropsExecutableMarkup(t *testing.T) {
	in := `<p onclick="x">保留<script>alert(1)</script><a href="java&#x73;cript:alert(1)" style="background:url(javascript:x)">链接</a><img alt="图片" src="//evil.test/x" srcset="x 1x" onerror="alert(2)"></p>` +
		`<form action="/x"><input name="q"><button formaction="/y">提交</button></form><iframe src="https://evil.test/f"></iframe><object data="x.swf"></object><embed src="x.swf">` +
		`<svg onload="alert(3)"><circle r="1"/></svg><p style="position:fixed;width:expression(alert(4));behavior:url(x.htc)">样式</p>`
	out := strings.ToLower(sanitizeWorkspaceHTML(in))
	for _, bad := range []string{"<script", "onclick", "onerror", "onload", "javascript", "srcset", "<form", "<input", "<button", "formaction", "<iframe", "<object", "<embed", "<svg", "evil.test", "alert", "expression", "behavior", "position"} {
		if strings.Contains(out, bad) {
			t.Fatalf("output contains %q: %s", bad, out)
		}
	}
	for _, text := range []string{"保留", "链接", "图片", "样式"} {
		if !strings.Contains(out, text) {
			t.Fatalf("text %q not preserved: %s", text, out)
		}
	}
}

func TestSanitizeWorkspaceHTMLKeepsTableAndInlineStyles(t *testing.T) {
	in := `<table width="600" cellpadding="0" cellspacing="0" border="0" bgcolor="#f2f3f5" align="center" style="max-width:560px;border-radius:8px">` +
		`<tr><th colspan="2">姓名</th><td bgcolor="#1f3a5f" valign="top" width="50%" class="col" style="padding:10px 22px;font:600 18px/26px Arial,sans-serif;color:#ffffff">备注</td></tr></table>` +
		`<div style="display:none;max-height:0;overflow:hidden">预览文字</div><font color="#333333" face="Arial">字体</font><unknown>中文<strong>加粗</strong></unknown>`
	out := sanitizeWorkspaceHTML(in)
	for _, want := range []string{
		`width="600"`, `cellpadding="0"`, `cellspacing="0"`, `border="0"`, `bgcolor="#f2f3f5"`, `align="center"`, `max-width: 560px`, `border-radius: 8px`,
		`<th colspan="2">姓名</th>`, `bgcolor="#1f3a5f"`, `valign="top"`, `width="50%"`, `class="col"`, `padding: 10px 22px`, `color: #ffffff`,
		`style="display: none; max-height: 0; overflow: hidden"`, `<font color="#333333" face="Arial">字体</font>`, "中文<strong>加粗</strong>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "unknown") || strings.Contains(out, "<html") {
		t.Fatalf("unknown wrapper preserved: %s", out)
	}
}

func TestSanitizeWorkspaceHTMLBlocksRemoteImagesAndOpensLinksInNewTab(t *testing.T) {
	out := sanitizeWorkspaceHTML(`<img src="https://img.example.test/hero.jpg" width="600" height="220" alt="秋季上新"><img src="data:image/png;base64,iVBORw0KGgo=" alt="内嵌"><a href="https://shop.example.test/new" target="_self" rel="opener">选购</a><a href="/relative">相对</a>`)
	for _, want := range []string{`<img src="" width="600" height="220" alt="秋季上新">`, `src="data:image/png;base64,iVBORw0KGgo="`, `<a href="https://shop.example.test/new" rel="nofollow noreferrer noopener" target="_blank">选购</a>`, "相对"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "img.example.test") || strings.Contains(out, `href="/relative"`) {
		t.Fatalf("remote image or relative link kept: %s", out)
	}
}

func TestSanitizeWorkspaceHTMLLinksKeepOnlyWebAndMailtoSchemes(t *testing.T) {
	in := `<a href="http://plain.example.test/a">明文</a><a href="mailto:hi@example.test">写信</a>` +
		`<a href="  javascript:alert(1)">脚本</a><a href="JaVaScRiPt:alert(2)">大小写</a><a href="java&#x73;cript:alert(3)">实体</a><a href="vbscript:x">VB</a><a href="file:///etc/passwd">文件</a>` +
		`<a href="data:text/html,<script>alert(4)</script>">网页</a><a href="DATA:image/png;base64,iVBORw0KGgo=">图链</a>` +
		`<img src="data:image/png;base64,iVBORw0KGgo=" usemap="#m"><map name="m"><area shape="rect" coords="0,0,1,1" href="data:image/png;base64,iVBORw0KGgo=" alt="热区"></map>`
	out := sanitizeWorkspaceHTML(in)
	for _, want := range []string{
		`<a href="http://plain.example.test/a" rel="nofollow noreferrer noopener" target="_blank">明文</a>`,
		`<a href="mailto:hi@example.test" rel="nofollow noreferrer">写信</a>`,
		`<a rel="nofollow noreferrer">图链</a>`, `<area shape="rect" coords="0,0,1,1" alt="热区" rel="nofollow noreferrer">`,
		`<img src="data:image/png;base64,iVBORw0KGgo=" usemap="#m">`,
		"脚本", "大小写", "实体", "VB", "文件", "网页",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	lower := strings.ToLower(out)
	for _, bad := range []string{"javascript", "vbscript", "file:", "alert", `href="data:`, "text/html"} {
		if strings.Contains(lower, bad) {
			t.Fatalf("output contains %q: %s", bad, out)
		}
	}
}

func TestSanitizeWorkspaceHTMLFixturesKeepLayout(t *testing.T) {
	for name, wants := range map[string][]string{
		"signin-notice": {`width="560"`, `style="display: none; max-height: 0; overflow: hidden"`, `color: #10a37f`, `border-top: 1px solid #ececf1`},
		"newsletter":    {`bgcolor="#1f3a5f"`, `width="600"`, `border-radius: 6px`, `class="col"`, `bgcolor="#e8590c"`},
	} {
		raw, err := os.ReadFile("testdata/mail/" + name + ".html")
		if err != nil {
			t.Fatal(err)
		}
		out := sanitizeWorkspaceHTML(string(raw))
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Fatalf("%s: missing %q in %s", name, want, out)
			}
		}
		if strings.Contains(out, "<style") || strings.Contains(out, "<title") || strings.Contains(out, "example.test/logo") || strings.Contains(out, "img.example.test") {
			t.Fatalf("%s: head content or remote image kept: %s", name, out)
		}
	}
}
