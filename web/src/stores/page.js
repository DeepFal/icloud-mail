import { reactive } from "vue";

const page = reactive({ title: "", subtitle: "" });
let unread = 0;

function renderTitle() {
  document.title = `${unread > 0 ? `(${unread}) ` : ""}${page.title ? `${page.title} - ` : ""}iCloud 隐私邮箱`;
}

export function setPageHeader(title, subtitle = "") {
  page.title = title || "";
  page.subtitle = subtitle || "";
  renderTitle();
}

// Shown in the tab title until the reporting page resets it to 0 when it unmounts.
export function setUnreadBadge(count) {
  unread = Math.max(0, Number(count) || 0);
  renderTitle();
}

export function usePageHeader() {
  return page;
}
