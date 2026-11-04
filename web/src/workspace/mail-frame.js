// Builds the srcdoc for the mail iframe. The frame is sandboxed without
// allow-scripts (allow-same-origin so the parent can measure it, popups so links
// open in a new tab), its CSP blocks every fetch except data: images, and the
// parent sizes it to its content so the reading pane stays the single scroller.

const LIGHT = { text: "#22252a", bg: "#ffffff", link: "#245fc5", border: "#e2e3e0", muted: "#666b72" };
const NEUTRAL_BG = /^(#fff(fff)?|white|transparent|none|inherit|initial|rgba?\(\s*255\s*,\s*255\s*,\s*255\b.*)$/i;

// Mail with its own non-white backgrounds or images is "designed";
// forcing dark colors onto it breaks its layout, so it stays on light paper.
export function isDesignedMail(html = "") {
  if ([...html.matchAll(/<img\b[^>]*>/gi)].some(([tag]) => !isPixel(tag))) return true;
  for (const match of html.matchAll(/bgcolor="([^"]*)"|background(?:-color)?:\s*([^;"]+)/gi)) {
    const color = (match[1] ?? match[2]).trim().match(/^(?:rgba?\([^)]*\)|\S+)/i)?.[0] || "";
    if (!NEUTRAL_BG.test(color)) return true;
  }
  return false;
}

// The server blanks remote image URLs to src="" or drops the attribute (never fetched): any
// <img> that is not a data: image is blocked. Input is sanitizer output: double-quoted, escaped values.
const BLOCKED_IMG = /<img\b(?![^>]*\ssrc="data:)[^>]*>/gi;
const isPixel = (tag) => /\s(?:width|height)="[0-2]"/i.test(tag);
const attr = (tag, name) => tag.match(new RegExp(`\\s${name}="([^"]*)"`, "i"))?.[1] ?? "";

// 1–2px tracking pixels do not count.
export function hasBlockedImages(html = "") {
  return [...String(html || "").matchAll(BLOCKED_IMG)].some(([tag]) => !isPixel(tag));
}

// "paper": the mail's own colors on a light sheet. "adapt": theme colors forced on.
export function mailRenderMode({ dark, designed }) {
  if (!dark) return "light";
  return designed ? "paper" : "adapt";
}

// Chrome draws a broken-image icon on any <img> it cannot load, with or without src, and page CSS
// cannot hide it. A blocked image becomes a dashed box with its alt text and size instead; images
// without alt text and tracking pixels are dropped. Mail cannot carry data-* (the sanitizer strips them).
function blockedImagePlaceholders(html = "") {
  return String(html || "").replace(BLOCKED_IMG, (tag) => {
    const alt = attr(tag, "alt").trim();
    if (!alt || isPixel(tag)) return "";
    const size = ["width", "height"].map((name) => [name, attr(tag, name)]).filter(([, value]) => /^\d+%?$/.test(value))
      .map(([name, value]) => `${name}:${value.endsWith("%") ? value : `${value}px`}`);
    const style = [...size, attr(tag, "style")].filter(Boolean).join(";");
    return `<span data-blocked${style ? ` style="${style}"` : ""}>${alt}</span>`;
  });
}

export function mailFrameDocument(html, { mode = "light", palette = LIGHT } = {}) {
  const adapt = mode === "adapt";
  const c = adapt ? palette : LIGHT;
  const force = adapt
    ? `body *{color:inherit!important;background-color:transparent!important;background-image:none!important;border-color:${c.border}!important}a,a *{color:${c.link}!important}`
    : "";
  const css = `html{color-scheme:${adapt ? "dark" : "light"};display:flow-root;overflow-x:auto;overflow-y:hidden}` +
    `body{margin:0;padding:16px 20px 24px;font:14px/1.6 -apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei","Segoe UI",sans-serif;color:${c.text};background:${c.bg};overflow-wrap:anywhere}` +
    `img{max-width:100%;height:auto}[data-blocked]{display:inline-block;box-sizing:border-box;max-width:100%;overflow:hidden;padding:2px 6px;outline:1px dashed ${c.border};outline-offset:-1px;color:${c.muted};font-size:12px;line-height:1.6;white-space:nowrap;text-overflow:ellipsis;vertical-align:middle}table{max-width:100%!important}pre{white-space:pre-wrap}a{color:${c.link}}` +
    `blockquote{margin:0 0 0 4px;padding-left:12px;border-left:2px solid ${c.border};color:${c.muted}}${force}`;
  return `<!doctype html><html><head><meta charset="utf-8"><meta name="referrer" content="no-referrer">` +
    `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'"><base target="_blank">` +
    `<style>${css}</style></head><body>${blockedImagePlaceholders(html)}</body></html>`;
}

// Content height of a loaded frame document; html is a flow root so floats and margins count.
// A horizontal scrollbar (wide fixed-width tables) adds its own height so nothing is clipped.
export function measureFrameHeight(doc) {
  const root = doc?.documentElement;
  if (!root || !doc.body?.childNodes?.length) return 0;
  const height = Math.ceil(root.getBoundingClientRect().height);
  const scrollbar = root.scrollWidth > root.clientWidth ? 16 : 0;
  return height > 0 ? height + scrollbar : 0;
}

export function readMailPalette(root = globalThis.document?.documentElement) {
  if (!root || typeof getComputedStyle !== "function") return LIGHT;
  const style = getComputedStyle(root), read = (name, fallback) => style.getPropertyValue(name).trim() || fallback;
  return { text: read("--text", LIGHT.text), bg: read("--surface", LIGHT.bg), link: read("--primary", LIGHT.link), border: read("--border", LIGHT.border), muted: read("--text-muted", LIGHT.muted) };
}
