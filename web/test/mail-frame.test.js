import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { hasBlockedImages, isDesignedMail, mailFrameDocument, mailRenderMode, measureFrameHeight, readRenderPreference, saveRenderPreference } from "../src/workspace/mail-frame.js";

const dark = { text: "#eceeef", bg: "#202124", link: "#8eb9ff", border: "#36383c", muted: "#a1a6ae" };

test("the frame document keeps the CSP lock-down and drops the old fixed white box", () => {
  const doc = mailFrameDocument("<p>正文</p>");
  assert.match(doc, /default-src 'none'; style-src 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'/);
  assert.match(doc, /<meta name="referrer" content="no-referrer">/);
  assert.match(doc, /base-uri 'none'"><base target="_blank">/, "links never navigate the frame itself");
  assert.match(doc, /<body><p>正文<\/p><\/body>/);
  assert.doesNotMatch(doc, /td,th\{border|line-height:1\.9|450px/);
  assert.match(doc, /html\{color-scheme:light;/);
});

test("adapt mode forces theme colors without inverting images", () => {
  const doc = mailFrameDocument("<p style=\"color:#333\">正文</p>", { mode: "adapt", palette: dark });
  assert.match(doc, /html\{color-scheme:dark;/);
  assert.match(doc, /color:#eceeef;background:#202124/);
  assert.match(doc, /body \*\{color:inherit!important;background-color:transparent!important/);
  assert.doesNotMatch(doc, /invert/);
  const paper = mailFrameDocument("<p>正文</p>", { mode: "paper", palette: dark });
  assert.match(paper, /color:#22252a;background:#ffffff/);
  assert.doesNotMatch(paper, /color:inherit!important|background-color:transparent!important/);
});

test("designed mail is recognised by backgrounds and inline images, not by white wrappers", () => {
  assert.equal(isDesignedMail('<table bgcolor="#ffffff"><tr><td style="background-color: #fff; color: #333">纯文字</td></tr></table>'), false);
  assert.equal(isDesignedMail("<p>你好</p>"), false);
  assert.equal(isDesignedMail('<td bgcolor="#1f3a5f">页眉</td>'), true);
  assert.equal(isDesignedMail('<div style="padding: 4px; background: #f2f3f5">卡片</div>'), true);
  assert.equal(isDesignedMail('<img src="data:image/png;base64,AAAA" alt="logo">'), true);
});

test("render mode: light theme renders as-is, plain mail adapts, designed mail follows the saved choice", () => {
  assert.equal(mailRenderMode({ dark: false, designed: true, preference: "adapt" }), "light");
  assert.equal(mailRenderMode({ dark: true, designed: false, preference: "paper" }), "adapt");
  assert.equal(mailRenderMode({ dark: true, designed: true, preference: "paper" }), "paper");
  assert.equal(mailRenderMode({ dark: true, designed: true, preference: "adapt" }), "adapt");
  const store = new Map();
  const storage = { getItem: (key) => store.get(key) ?? null, setItem: (key, value) => store.set(key, value) };
  assert.equal(readRenderPreference(storage), "paper");
  saveRenderPreference("adapt", storage);
  assert.equal(readRenderPreference(storage), "adapt");
  assert.equal(readRenderPreference({ getItem() { throw new Error("denied"); } }), "paper");
});

test("blocked remote images are reported, tracking pixels are not", () => {
  assert.equal(hasBlockedImages('<img src="" width="600" height="220" alt="秋季上新">'), true);
  assert.equal(hasBlockedImages('<p>x</p><img alt="logo" src="">'), true, "src is not always the first attribute");
  assert.equal(hasBlockedImages('<img alt="协议相对">'), true, "protocol-relative URLs lose src entirely");
  assert.equal(hasBlockedImages('<img src="" width="1" height="1" alt="">'), false);
  assert.equal(hasBlockedImages('<img src="data:image/png;base64,AAAA" alt="x">'), false);
});

test("the inbox frame never runs scripts and is sized by the parent inside one scroller", async () => {
  const [view, css] = await Promise.all([
    readFile(new URL("../src/workspace/InboxView.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/workspace/inbox.css", import.meta.url), "utf8"),
  ]);
  const frames = view.match(/<iframe[^>]*>/g) || [];
  assert.equal(frames.length, 1);
  const sandbox = frames[0].match(/\ssandbox="([^"]*)"/)?.[1].split(/\s+/);
  assert.deepEqual(sandbox, ["allow-same-origin", "allow-popups", "allow-popups-to-escape-sandbox"]);
  assert.ok(!sandbox.includes("allow-scripts"));
  assert.doesNotMatch(frames[0], /:sandbox/, "the sandbox value is static");
  assert.doesNotMatch(view, /allow-scripts|allow-forms|allow-modals|allow-top-navigation/);
  assert.match(frames[0], /:srcdoc="frameDocument"/);
  assert.match(frames[0], /@load="fitFrame"/);
  assert.match(frames[0], /height: `\$\{frameHeight\}px`/);
  assert.doesNotMatch(view, /v-html/);
  assert.doesNotMatch(css, /height:\s*450px|background:\s*#fff\b|filter:\s*invert/);
  assert.match(css, /\.reader-scroll \{[^}]*overflow: auto/);
  assert.doesNotMatch(css.match(/\.html-body \{[^}]*\}/)[0], /overflow|flex: 1/);
  assert.doesNotMatch(css.match(/\.text-body \{[^}]*\}/)[0], /overflow: auto/);
});

test("frame height comes from the loaded document, including a horizontal scrollbar", () => {
  const doc = (height, scrollWidth = 600, clientWidth = 600, children = 1) => ({ body: { childNodes: { length: children } }, documentElement: { getBoundingClientRect: () => ({ height }), scrollWidth, clientWidth } });
  assert.equal(measureFrameHeight(doc(812.4)), 813);
  assert.equal(measureFrameHeight(doc(500, 640, 600)), 516);
  assert.equal(measureFrameHeight(doc(300, 600, 600, 0)), 0, "about:blank before srcdoc loads");
  assert.equal(measureFrameHeight(undefined), 0);
  assert.match(mailFrameDocument("<p>x</p>"), /html\{color-scheme:light;display:flow-root;overflow-x:auto;overflow-y:hidden\}/);
});

test("blocked remote images become alt-text boxes, never an <img> Chrome draws a broken icon for", () => {
  const body = mailFrameDocument(
    '<img src="" width="600" height="220" alt="秋季上新"><img alt="logo" src=""><img alt="协议相对"/>' +
    '<img src="" width="50%" style="display: block; height: auto" alt="半宽">' +
    '<img src="" width="120" height="40"><img src="" alt=" "><img src="" width="1" height="1" alt="像素">' +
    '<img src="data:image/png;base64,AAAA" alt="内嵌">',
  ).match(/<body>(.*)<\/body>/)[1];
  assert.equal(body,
    '<span data-blocked style="width:600px;height:220px">秋季上新</span><span data-blocked>logo</span><span data-blocked>协议相对</span>' +
    '<span data-blocked style="width:50%;display: block; height: auto">半宽</span>' +
    '<img src="data:image/png;base64,AAAA" alt="内嵌">');
  assert.match(mailFrameDocument(""), /\[data-blocked\]\{display:inline-block;[^}]*outline:1px dashed/);
});
