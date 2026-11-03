import assert from "node:assert/strict";
import test from "node:test";
import { nextTick, reactive, ref } from "vue";
import { useInboxMessages, watchInboxFilters } from "../src/workspace/inbox-state.js";

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function fixture(options = {}) {
  const stored = Array.from({ length: options.count || 3 }, (_, i) => ({ aliasId: 1, mailboxUid: i + 1, subject: `邮件 ${i + 1}`, read: false, starred: true, archived: false, deleted: false }));
  const filters = reactive({ folder: "inbox", unread: true, starred: false });
  const page = ref(options.page || 1), errors = [], calls = [];
  const api = {
    async getMessages({ offset = 0, limit = 40, ...query }) {
      let rows = stored.filter((mail) => (!query.unread || !mail.read) && (!query.starred || mail.starred) && (query.folder === "trash" ? mail.deleted : !mail.deleted && (query.folder === "archived" ? mail.archived : !mail.archived)));
      return { items: rows.slice(offset, offset + limit).map((mail) => ({ ...mail })), total: rows.length, unreadCount: rows.filter((mail) => !mail.read).length };
    },
    async getMessage(aliasId, uid) { return { text: `正文 ${uid}`, html: "", contentAvailable: true, attachments: [] }; },
    async patchMessage(aliasId, uid, patch, csrf) {
      calls.push({ aliasId, uid, patch, csrf });
      const mail = stored.find((item) => item.mailboxUid === uid);
      Object.assign(mail, patch);
      return { ...mail };
    },
    async batchMessages(action, mails) {
      for (const mail of mails) stored.find((item) => item.mailboxUid === mail.mailboxUid).read = action === "read";
      return { action, updated: mails.length };
    },
    ...options.api,
  };
  const state = useInboxMessages({ filters, page, pageSize: options.pageSize || 40, getMessages: (...args) => api.getMessages(...args), getMessage: (...args) => api.getMessage(...args), patchMessage: (...args) => api.patchMessage(...args), batchMessages: (...args) => api.batchMessages(...args), getCsrfToken: () => "csrf-fixture", onError: (message) => errors.push(message) });
  return { state, stored, filters, page, errors, calls, api };
}

test("opening unread mail preserves its body and mobile reader after it leaves the results", async () => {
  const { state, calls } = fixture();
  await state.loadList();
  await state.selectMessage(state.messages.value[0]);
  assert.equal(state.selected.value.mailboxUid, 1);
  assert.equal(state.selected.value.read, true);
  assert.equal(state.detail.value.text, "正文 1");
  assert.equal(state.readerOpen.value, true);
  assert.deepEqual(state.messages.value.map((mail) => mail.mailboxUid), [2, 3]);
  assert.equal(state.nextMessage.value.mailboxUid, 2);
  assert.equal(state.previousMessage.value, undefined);
  assert.equal(state.total.value, 2);
  assert.equal(calls[0].csrf, "csrf-fixture");
  await state.loadList({ silent: true });
  assert.equal(state.selected.value.mailboxUid, 1);
  assert.equal(state.readerOpen.value, true);
  state.dispose();
});

test("reading the final unread message keeps the reader open even with zero results", async () => {
  const { state } = fixture({ count: 1 });
  await state.loadList();
  await state.selectMessage(state.messages.value[0]);
  assert.equal(state.messages.value.length, 0);
  assert.equal(state.selected.value.mailboxUid, 1);
  assert.equal(state.detail.value.text, "正文 1");
  assert.equal(state.readerOpen.value, true);
  state.dispose();
});

test("shrinking the last results page does not dismiss the current reader", async () => {
  const { state, page } = fixture({ count: 3, page: 2, pageSize: 2 });
  await state.loadList();
  await state.selectMessage(state.messages.value[0]);
  assert.equal(page.value, 1);
  assert.equal(state.selected.value.mailboxUid, 3);
  assert.equal(state.detail.value.text, "正文 3");
  assert.equal(state.readerOpen.value, true);
  assert.deepEqual(state.messages.value.map((mail) => mail.mailboxUid), [1, 2]);
  state.dispose();
});

test("removing a star keeps the current body while the starred results update", async () => {
  const { state, filters } = fixture();
  filters.unread = false; filters.starred = true;
  await state.loadList();
  const mail = state.selected.value;
  await state.selectMessage(mail);
  await state.mutate({ starred: false }, mail);
  assert.equal(state.selected.value.starred, false);
  assert.equal(state.detail.value.text, "正文 1");
  assert.equal(state.readerOpen.value, true);
  assert.equal(state.messages.value.length, 2);
  state.dispose();
});

test("changing filters invalidates an in-flight body instead of restoring the old reader", async () => {
  const request = deferred();
  const { state, api } = fixture();
  await state.loadList();
  api.getMessage = () => request.promise;
  const pending = state.selectMessage(state.messages.value[0]);
  state.resetReader();
  request.resolve({ text: "旧正文", html: "" });
  await pending;
  assert.equal(state.selected.value, null);
  assert.equal(state.detail.value, null);
  assert.equal(state.readerOpen.value, false);
  state.dispose();
});

test("a completed archive on another mail cannot close the newer selected message", async () => {
  const { state, api, stored, filters } = fixture();
  filters.unread = false;
  await state.loadList();
  const first = state.messages.value[0], second = state.messages.value[1];
  const request = deferred();
  const patch = api.patchMessage;
  api.patchMessage = async (...args) => { await request.promise; return patch(...args); };
  const moving = state.mutate({ archived: true }, first);
  await state.selectMessage(second, { markRead: false });
  request.resolve(); await moving;
  assert.equal(stored[0].archived, true);
  assert.equal(state.selected.value.mailboxUid, 2);
  assert.equal(state.detail.value.text, "正文 2");
  assert.equal(state.readerOpen.value, true);
  state.dispose();
});

test("failed state writes leave the mail unchanged and a failed body can be retried", async () => {
  const { state, api, errors, calls } = fixture();
  await state.loadList();
  api.patchMessage = async () => { throw new Error("连接已中断"); };
  await state.selectMessage(state.selected.value);
  assert.equal(state.selected.value.read, false);
  assert.equal(state.readerOpen.value, true);
  assert.equal(state.detail.value.text, "正文 1");
  assert.equal(state.busy.size, 0);
  assert.deepEqual(errors, ["连接已中断"]);
  api.getMessage = async () => { throw new Error("正文暂不可用"); };
  await state.selectMessage(state.selected.value);
  assert.equal(state.detailError.value, "正文暂不可用");
  api.getMessage = async () => ({ text: "已恢复的正文", html: "" });
  await state.selectMessage(state.selected.value, { markRead: false });
  assert.equal(state.detailError.value, "");
  assert.equal(state.detail.value.text, "已恢复的正文");
  assert.equal(calls.length, 0);
  state.dispose();
});

test("a stale poll cannot restore pre-mutation unread state", async () => {
  const { state, api } = fixture();
  await state.loadList();
  const originalList = api.getMessages;
  const old = await originalList({ ...reactive({ folder: "inbox", unread: true }) });
  const request = deferred();
  api.getMessages = () => request.promise;
  const polling = state.loadList({ silent: true });
  api.getMessages = originalList;
  await state.selectMessage(state.selected.value);
  request.resolve(old); await polling;
  assert.equal(state.selected.value.read, true);
  assert.equal(state.messages.value.some((mail) => mail.mailboxUid === 1), false);
  state.dispose();
});

test("disposing a pending reader cannot send an automatic read write", async () => {
  const request = deferred();
  const { state, api, calls } = fixture();
  await state.loadList();
  api.getMessage = () => request.promise;
  const pending = state.selectMessage(state.selected.value);
  state.dispose(); request.resolve({ text: "late", html: "" }); await pending;
  assert.equal(state.selected.value, null);
  assert.equal(calls.length, 0);
});

test("batch read updates the open reader that leaves the unread results", async () => {
  const { state, filters, api } = fixture();
  await state.loadList();
  assert.equal(state.selected.value.mailboxUid, 1);
  assert.equal(state.selected.value.read, false);
  assert.equal(await state.batch("read", state.messages.value.slice(0, 2)), true);
  assert.deepEqual(state.messages.value.map((mail) => mail.mailboxUid), [3]);
  assert.equal(state.selected.value.mailboxUid, 1, "the reader keeps the mail that left the results");
  assert.equal(state.selected.value.read, true);
  assert.equal(state.detail.value.text, "正文 1");

  filters.unread = false;
  await state.loadList();
  api.getMessages = async () => { throw new Error("列表暂时不可用"); };
  assert.equal(await state.batch("unread", [state.selected.value]), true);
  assert.equal(state.selected.value.read, false, "the reader shows the confirmed state even when the list refresh fails");
  state.dispose();
});

test("search waits for a typing pause while other filters reload at once", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const filters = reactive({ folder: "inbox", query: "", accountId: "", aliasId: "", groupId: "", unread: false, starred: false, hasOtp: false });
  let changes = 0, reloads = 0;
  const stop = watchInboxFilters(filters, { onChange: () => { changes++; }, reload: () => { reloads++; } });
  for (const query of ["v", "ve", "ver"]) {
    filters.query = query;
    await nextTick();
    t.mock.timers.tick(200);
  }
  assert.equal(changes, 3);
  assert.equal(reloads, 0, "keystrokes inside the delay must not request");
  t.mock.timers.tick(49);
  assert.equal(reloads, 0);
  t.mock.timers.tick(1);
  assert.equal(reloads, 1, "one request after the pause");

  filters.hasOtp = true;
  await nextTick();
  assert.equal(reloads, 2, "non-search filters reload immediately");
  filters.query = "x"; filters.folder = "trash";
  await nextTick();
  assert.equal(reloads, 3, "a mixed change reloads immediately");
  t.mock.timers.tick(1000);
  assert.equal(reloads, 3, "no debounced search is left behind");

  filters.query = "y";
  await nextTick();
  stop();
  t.mock.timers.tick(1000);
  assert.equal(reloads, 3, "stopping cancels a pending search");
});
