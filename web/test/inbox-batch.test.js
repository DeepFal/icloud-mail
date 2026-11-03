import assert from "node:assert/strict";
import test from "node:test";
import { reactive, ref } from "vue";
import { formatMailDate } from "../src/workspace/mail-format.js";
import { batchMessages, getMessages } from "../src/workspace/mail-api.js";
import { useInboxMessages } from "../src/workspace/inbox-state.js";
import { setPageHeader, setUnreadBadge } from "../src/stores/page.js";

test("mail dates show the year only for mail from other years", () => {
  const now = new Date(2026, 10, 3, 9, 0);
  assert.equal(formatMailDate(new Date(2025, 2, 5, 14, 30), now), "2025-03-05");
  assert.equal(formatMailDate(new Date(2024, 11, 31, 23, 59), now), "2024-12-31");
  assert.doesNotMatch(formatMailDate(new Date(2026, 0, 2, 8, 5), now), /2026|-/);
  assert.equal(formatMailDate("not a date", now), "时间未知");
});

test("tab title carries the unread count and drops it at zero", () => {
  globalThis.document = {};
  setPageHeader("收件箱");
  assert.equal(document.title, "收件箱 - iCloud 隐私邮箱");
  setUnreadBadge(3);
  assert.equal(document.title, "(3) 收件箱 - iCloud 隐私邮箱");
  setPageHeader("收件箱");
  assert.equal(document.title, "(3) 收件箱 - iCloud 隐私邮箱");
  setUnreadBadge(0);
  assert.equal(document.title, "收件箱 - iCloud 隐私邮箱");
  delete globalThis.document;
});

test("batch and has_otp requests use the documented shapes and CSRF", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, options = {}) => {
    calls.push({ url: String(url), options });
    return new Response(JSON.stringify({ data: options.method === "POST" ? { action: "read", updated: 2 } : { items: [], total: 0 } }), { status: 200, headers: { "content-type": "application/json" } });
  });
  await getMessages({ hasOtp: true });
  assert.match(calls[0].url, /has_otp=1/);
  await batchMessages("archive", [{ aliasId: "2", mailboxUid: 9 }, { aliasId: 3, mailboxUid: "4" }], "csrf-fixture");
  assert.match(calls[1].url, /\/messages\/batch$/);
  assert.equal(calls[1].options.method, "POST");
  assert.equal(calls[1].options.headers.get("X-CSRF-Token"), "csrf-fixture");
  assert.deepEqual(JSON.parse(calls[1].options.body), { action: "archive", items: [{ alias_id: 2, mailbox_uid: 9 }, { alias_id: 3, mailbox_uid: 4 }] });
});

function inbox(batchMessagesImpl) {
  const stored = [1, 2, 3].map((uid) => ({ aliasId: 1, mailboxUid: uid, read: false, archived: false }));
  const errors = [], calls = [];
  const state = useInboxMessages({
    filters: reactive({ folder: "inbox" }), page: ref(1),
    getMessages: async () => ({ items: stored.filter((mail) => !mail.archived).map((mail) => ({ ...mail })), total: stored.filter((mail) => !mail.archived).length, unreadCount: stored.filter((mail) => !mail.read).length }),
    getMessage: async () => ({ text: "正文", html: "", contentAvailable: true, attachments: [] }),
    patchMessage: async () => ({}),
    batchMessages: async (action, mails, csrf) => { calls.push({ action, uids: mails.map((m) => m.mailboxUid), csrf }); return batchMessagesImpl(action, mails, stored); },
    getCsrfToken: () => "csrf-fixture", onError: (message) => errors.push(message),
  });
  return { state, stored, errors, calls };
}

test("batch archive refreshes the list and closes the reader when the open mail moved", async () => {
  const { state, calls } = inbox(async (action, mails, stored) => { for (const mail of mails) stored.find((m) => m.mailboxUid === mail.mailboxUid).archived = true; });
  await state.loadList();
  await state.selectMessage(state.messages.value[0], { open: true, markRead: false });
  assert.equal(await state.batch("archive", state.messages.value.slice(0, 2)), true);
  assert.deepEqual(calls, [{ action: "archive", uids: [1, 2], csrf: "csrf-fixture" }]);
  assert.deepEqual(state.messages.value.map((m) => m.mailboxUid), [3]);
  assert.equal(state.readerOpen.value, false);
  assert.equal(state.busy.size, 0);
  state.dispose();
});

test("failed batch reports the error, keeps the list and releases busy keys", async () => {
  const { state, errors } = inbox(async () => { throw new Error("部分邮件不存在，本次未做任何修改"); });
  await state.loadList();
  assert.equal(await state.batch("trash", state.messages.value), false);
  assert.deepEqual(errors, ["部分邮件不存在，本次未做任何修改"]);
  assert.equal(state.messages.value.length, 3);
  assert.equal(state.busy.size, 0);
  assert.equal(await state.batch("read", []), false);
  state.dispose();
});
