// Builds the srcdoc for the mail iframe. The frame is sandboxed without
// allow-scripts (only allow-same-origin, so the parent can measure it), its CSP
// blocks every fetch except data: images, and the parent sizes it to its
// content so the reading pane stays the single scroll container.

export const MAIL_RENDER_KEY = "icloud-mail-dark-render";
const LIGHT = { text: "#22252a", bg: "#ffffff", link: "#245fc5", border: "#e2e3e0", muted: "#666b72" };
const NEUTRAL_BG = /^(#fff(fff)?|white|transparent|none|inherit|initial|rgba?\(\s*255\s*,\s*255\s*,\s*255\b.*)$/i;

// Mail with its own non-white backgrounds or inline images is "designed";
// forcing dark colors onto it breaks its layout, so it stays on light paper.
export function isDesignedMail(html = "") {
  if (/<img[^>]*\ssrc="data:/i.test(html)) return true;
  for (const match of html.matchAll(/bgcolor="([^"]*)"|background(?:-color)?:\s*([^;"]+)/gi)) {
    if (!NEUTRAL_BG.test((match[1] ?? match[2]).trim().split(/\s+/)[0])) return true;
  }
  return false;
}

// Remote images arrive with src="" (never fetched); 1–2px tracking pixels do not count.
export function hasBlockedImages(html = "") {
  return [...html.matchAll(/<img src=""[^>]*>/gi)].some(([tag]) => !/\b(width|height)="[0-2]"/.test(tag));
}

// "paper": the mail's own colors on a light sheet. "adapt": theme colors forced on.
export function mailRenderMode({ dark, designed, preference }) {
  if (!dark) return "light";
  if (!designed) return "adapt";
  return preference === "adapt" ? "adapt" : "paper";
}

// The server blanks remote image URLs to src=""; without the attribute browsers show the alt text instead of a broken icon.
function withoutEmptySources(html = "") {
  return String(html || "").replace(/(<img\b[^>]*?)\ssrc=""/gi, "$1 data-blocked");
}

export function mailFrameDocument(html, { mode = "light", palette = LIGHT } = {}) {
  const adapt = mode === "adapt";
  const c = adapt ? palette : LIGHT;
  const force = adapt
    ? `body *{color:inherit!important;background-color:transparent!important;background-image:none!important;border-color:${c.border}!important}a,a *{color:${c.link}!important}`
    : "";
  const css = `html{color-scheme:${adapt ? "dark" : "light"};display:flow-root;overflow-x:auto;overflow-y:hidden}` +
    `body{margin:0;padding:16px 20px 24px;font:14px/1.6 -apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei","Segoe UI",sans-serif;color:${c.text};background:${c.bg};overflow-wrap:anywhere}` +
    `img{max-width:100%;height:auto}img[data-blocked]{display:inline-block;min-width:1em;min-height:1.6em;padding:2px 6px;outline:1px dashed ${c.border};outline-offset:-1px;color:${c.muted};font-size:12px;vertical-align:middle}img[data-blocked]:not([alt]),img[data-blocked][alt=""]{display:none}table{max-width:100%!important}pre{white-space:pre-wrap}a{color:${c.link}}` +
    `blockquote{margin:0 0 0 4px;padding-left:12px;border-left:2px solid ${c.border};color:${c.muted}}${force}`;
  return `<!doctype html><html><head><meta charset="utf-8"><meta name="referrer" content="no-referrer">` +
    `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'">` +
    `<style>${css}</style></head><body>${withoutEmptySources(html)}</body></html>`;
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

export function readRenderPreference(storage = globalThis.localStorage) {
  try { return storage?.getItem(MAIL_RENDER_KEY) === "adapt" ? "adapt" : "paper"; } catch { return "paper"; }
}

export function saveRenderPreference(value, storage = globalThis.localStorage) {
  try { storage?.setItem(MAIL_RENDER_KEY, value === "adapt" ? "adapt" : "paper"); } catch { /* storage is optional */ }
}
