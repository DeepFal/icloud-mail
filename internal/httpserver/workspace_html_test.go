package httpserver

import (
	"strings"
	"testing"
)

func TestSanitizeWorkspaceHTMLDropsExecutableMarkup(t *testing.T) {
	in := `<p onclick="x">保留<script>alert(1)</script><a href="java&#x73;cript:alert(1)" style="background:url(javascript:x)">链接</a><img alt="图片" src="//evil.test/x" srcset="x 1x"></p><form formaction="/x">表单</form>`
	out := sanitizeWorkspaceHTML(in)
	for _, bad := range []string{"script", "onclick", "href", "style", "src", "srcset", "formaction", "evil.test", "alert"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Fatalf("output contains %q: %s", bad, out)
		}
	}
	if !strings.Contains(out, "保留") || !strings.Contains(out, "链接") || !strings.Contains(out, "图片") {
		t.Fatalf("text not preserved: %s", out)
	}
}

func TestSanitizeWorkspaceHTMLKeepsTableAndSafeSpans(t *testing.T) {
	in := `<table style="x"><tr><th colspan="2">姓名</th><td rowspan="101">坏跨度</td><td rowspan="2">备注</td></tr></table><unknown>中文<strong>加粗</strong></unknown>`
	out := sanitizeWorkspaceHTML(in)
	if !strings.Contains(out, `<th colspan="2">姓名</th>`) || !strings.Contains(out, `<td rowspan="2">备注</td>`) {
		t.Fatalf("table not preserved: %s", out)
	}
	if strings.Contains(out, `rowspan="101"`) || strings.Contains(out, "style") || strings.Contains(out, "unknown") || strings.Contains(out, "<html") {
		t.Fatalf("unsafe wrapper/attribute preserved: %s", out)
	}
	if !strings.Contains(out, "中文<strong>加粗</strong>") {
		t.Fatalf("unknown tag text not preserved: %s", out)
	}
}
