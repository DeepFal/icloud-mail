import { computed, reactive, ref } from "vue";

export const messageKey = (mail) => mail ? `${mail.aliasId}:${mail.mailboxUid}` : "";

// List membership and the open reader have separate lifetimes. In particular,
// marking a message read must not dismiss it from an unread-only reading flow.
export function useInboxMessages({ filters, page, pageSize = 40, getMessages, getMessage, patchMessage, getCsrfToken, onError }) {
  const messages = ref([]), selected = ref(null), detail = ref(null);
  const readerOpen = ref(false), viewMode = ref("text");
  const loading = ref(false), detailLoading = ref(false);
  const error = ref(""), detailError = ref("");
  const total = ref(0), unreadCount = ref(0), selectionOffset = ref(0);
  const busy = reactive(new Set());
  let listController, detailController, listVersion = 0, detailVersion = 0, disposed = false;

  const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)));
  const selectedIndex = computed(() => messages.value.findIndex((mail) => messageKey(mail) === messageKey(selected.value)));
  const previousMessage = computed(() => messages.value[(selectedIndex.value < 0 ? Math.min(selectionOffset.value, messages.value.length) : selectedIndex.value) - 1]);
  const nextMessage = computed(() => messages.value[selectedIndex.value < 0 ? selectionOffset.value : selectedIndex.value + 1]);

  function cancelList() {
    listVersion++;
    listController?.abort();
    loading.value = false;
  }

  function resetReader() {
    detailVersion++;
    detailController?.abort();
    selected.value = null;
    detail.value = null;
    detailError.value = "";
    detailLoading.value = false;
    readerOpen.value = false;
  }

  async function loadList({ silent = false, keep = true } = {}) {
    const version = ++listVersion;
    listController?.abort();
    listController = new AbortController();
    if (!silent) loading.value = true;
    error.value = "";
    try {
      const result = await getMessages({ ...filters, offset: (page.value - 1) * pageSize, limit: pageSize, signal: listController.signal });
      if (disposed || version !== listVersion) return false;
      messages.value = result.items;
      total.value = result.total;
      unreadCount.value = result.unreadCount;
      if (page.value > pages.value) {
        page.value = pages.value;
        return loadList({ silent, keep });
      }
      const current = keep && messages.value.find((mail) => messageKey(mail) === messageKey(selected.value));
      if (current) { selected.value = current; selectionOffset.value = selectedIndex.value; }
      else if (keep && selected.value) { /* Keep the open message even after it leaves these results. */ }
      else if (messages.value[0]) await selectMessage(messages.value[0], { open: false, markRead: false });
      else resetReader();
      return true;
    } catch (cause) {
      if (cause.name !== "AbortError" && version === listVersion && !disposed) error.value = cause.message || "邮件加载失败，请重新加载。";
      return false;
    } finally {
      if (version === listVersion && !disposed) loading.value = false;
    }
  }

  async function selectMessage(mail, { open = true, markRead = true } = {}) {
    if (!mail || disposed) return;
    const version = ++detailVersion;
    detailController?.abort();
    detailController = new AbortController();
    const index = messages.value.findIndex((item) => messageKey(item) === messageKey(mail));
    if (index >= 0) selectionOffset.value = index;
    selected.value = mail;
    detail.value = null;
    detailError.value = "";
    detailLoading.value = true;
    if (open) readerOpen.value = true;
    try {
      const result = await getMessage(mail.aliasId, mail.mailboxUid, { signal: detailController.signal });
      if (disposed || version !== detailVersion) return;
      detail.value = result;
      viewMode.value = result.html ? "html" : "text";
      if (markRead && !mail.read) await mutate({ read: true }, mail);
    } catch (cause) {
      if (cause.name !== "AbortError" && version === detailVersion && !disposed) detailError.value = cause.message || "正文读取失败，请重试。";
    } finally {
      if (version === detailVersion && !disposed) detailLoading.value = false;
    }
  }

  async function mutate(patch, mail = selected.value) {
    if (!mail || disposed) return false;
    const key = messageKey(mail);
    if (busy.has(key)) return false;
    busy.add(key);
    try {
      const result = await patchMessage(mail.aliasId, mail.mailboxUid, patch, getCsrfToken());
      if (disposed) return false;
      // A poll started before this write must not put the old state back.
      cancelList();
      Object.assign(mail, result);
      const row = messages.value.find((item) => messageKey(item) === key);
      if (row && row !== mail) Object.assign(row, result);
      const isSelected = messageKey(selected.value) === key;
      if (isSelected) Object.assign(selected.value, result);
      const movesFolder = "archived" in patch || "deleted" in patch;
      if (movesFolder && isSelected) resetReader();
      await loadList({ silent: true, keep: !movesFolder || !isSelected });
      return true;
    } catch (cause) {
      if (!disposed) onError(cause.message || "状态保存失败，请重试。原状态已保留。");
      return false;
    } finally {
      busy.delete(key);
    }
  }

  function dispose() {
    disposed = true;
    cancelList();
    resetReader();
  }

  return { messages, selected, detail, readerOpen, viewMode, loading, detailLoading, error, detailError, total, unreadCount, busy, pages, selectedIndex, previousMessage, nextMessage, loadList, selectMessage, mutate, cancelList, resetReader, dispose };
}
