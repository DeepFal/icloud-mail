package httpserver

import (
	"os"
	"strings"
	"testing"
)

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

func TestSanitizeWorkspaceHTMLImagesAndLinksStayInert(t *testing.T) {
	out := sanitizeWorkspaceHTML(`<img src="https://img.example.test/hero.jpg" width="600" height="220" alt="秋季上新"><img src="data:image/png;base64,iVBORw0KGgo=" alt="内嵌"><a href="https://shop.example.test/new">选购</a><a href="/relative">相对</a>`)
	for _, want := range []string{`<img src="" width="600" height="220" alt="秋季上新">`, `src="data:image/png;base64,iVBORw0KGgo="`, `target="_blank"`, `noreferrer`, `noopener`, "相对"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "img.example.test") || strings.Contains(out, `href="/relative"`) {
		t.Fatalf("remote image or relative link kept: %s", out)
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
