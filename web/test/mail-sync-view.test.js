import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { reactive, ref } from "vue";
import { syncAccount } from "../src/api/admin.js";
import { createLiveRefresh } from "../src/utils/liveRefresh.js";
import { useInboxReceive } from "../src/workspace/inbox-receive.js";
import { useInboxMessages } from "../src/workspace/inbox-state.js";
import { batchMessages, getMessage, getMessages, patchMessage } from "../src/workspace/mail-api.js";

const syncStatusPath = new URL(
  "../src/components/SyncStatus.vue",
  import.meta.url,
);
const accountDetailPath = new URL(
  "../src/views/AccountDetailView.vue",
  import.meta.url,
);
const accountsViewPath = new URL(
  "../src/views/AccountsView.vue",
  import.meta.url,
);

test("shared sync status renders manual and automatic progress", async () => {
  const source = await readFile(syncStatusPath, "utf8");

  assert.match(source, /syncProgressPresentation\(props\.item\.syncProgress\)/);
  assert.match(source, /v-if="progress\.active"/);
  assert.match(source, /<el-progress/);
  assert.match(source, /:indeterminate="progress\.indeterminate"/);
  assert.match(
    source,
    /:aria-hidden="progress\.indeterminate \? 'true' : undefined"/,
  );
  assert.match(source, /progress\.stageLabel/);
  assert.match(source, /progress\.percentage/);
});

test("account list and detail share server-driven sync progress", async () => {
  const [detailSource, listSource] = await Promise.all([
    readFile(accountDetailPath, "utf8"),
    readFile(accountsViewPath, "utf8"),
  ]);

  assert.match(detailSource, /const syncActive = computed\(/);
  assert.match(detailSource, /account\.value\?\.syncProgress\?\.active/);
  assert.match(detailSource, /:loading="syncLoading \|\| syncActive"/);
  assert.match(
    detailSource,
    /if \([^\n]*syncLoading\.value \|\| syncActive\.value \|\| randomAliasLoading\.value[^\n]*\) return/,
  );
  assert.ok(
    (listSource.match(/<SyncStatus :item=/g) || []).length >= 2,
    "desktop and mobile account lists must use the shared sync status",
  );
});

test("existing sync error summaries open the reusable full log dialog", async () => {
  const [statusSource, detailSource] = await Promise.all([
    readFile(syncStatusPath, "utf8"),
    readFile(accountDetailPath, "utf8"),
  ]);

  assert.match(
    statusSource,
    /错误：\{\{ compactRunes\(item\.lastSyncError\) \}\}[\s\S]*<SyncErrorLogDialog/,
  );
  assert.match(
    detailSource,
    /\{\{ account\.lastSyncError \}\}[\s\S]*<SyncErrorLogDialog/,
  );
  assert.match(
    detailSource,
    /account\.lastSyncErrorLog \|\| account\.lastSyncError/,
  );
});

const inboxViewPath = new URL("../src/workspace/InboxView.vue", import.meta.url);
const inboxReceivePath = new URL("../src/workspace/inbox-receive.js", import.meta.url);

test("inbox refresh, mount, polling and filters only reload the local list; 收取 is a separate button", async () => {
  const [source, receiveSource] = await Promise.all([readFile(inboxViewPath, "utf8"), readFile(inboxReceivePath, "utf8")]);
  assert.equal((source.match(/<Refresh :class/g) || []).length, 1);
  assert.match(source, /class="list-refresh" :disabled="loading" @click="loadList\(\)"/);
  assert.match(source, /createLiveRefresh\(\(\) => loadList\(\{ silent: true \}\)\)/);
  assert.match(source, /onMounted\(\(\) => \{ loadMeta\(\); loadList\(\); liveRefresh\.start\(/);
  // Only the explicit 收取 button reaches receive(); syncAccount is just handed to the receiver.
  assert.match(source, /<button class="list-receive" [^>]*@click="receive">\{\{ receiving \? '提交中…' : '收取' \}\}<\/button>/);
  assert.equal((source.match(/"receive"/g) || []).length, 1, "receive is bound only to the 收取 button");
  assert.doesNotMatch(source, /[^-\w]receive\(/, "nothing in the script calls receive() implicitly");
  assert.equal((source.match(/\bsyncAccount\b/g) || []).length, 2, "imported once and passed once to useInboxReceive");
  assert.doesNotMatch(source, /\/sync\b|checkUpstream|上游检查|完成于|method:/);
  assert.doesNotMatch(receiveSource, /完成/);
});

test("inbox list reads never submit receiving to an account", async (t) => {
  const requests = [];
  t.mock.method(globalThis, "fetch", async (url, options = {}) => {
    requests.push(`${options.method || "GET"} ${new URL(String(url), "http://local").pathname}`);
    const data = { items: [{ alias_id: 1, mailbox_uid: 1, subject: "邮件" }], total: 1, unread_count: 1 };
    return new Response(JSON.stringify({ data }), { status: 200, headers: { "content-type": "application/json" } });
  });
  const filters = reactive({ folder: "inbox", query: "", accountId: "", aliasId: "", groupId: "", unread: false, starred: false, hasOtp: false });
  const state = useInboxMessages({ filters, page: ref(1), getMessages, getMessage, patchMessage, batchMessages, getCsrfToken: () => "csrf-fixture", onError: assert.fail });
  const timers = [];
  const poll = createLiveRefresh(() => state.loadList({ silent: true }), { setTimeoutFn: (callback) => timers.push(callback), clearTimeoutFn: () => {}, documentTarget: null, windowTarget: null, navigatorTarget: null });

  await state.loadList(); // mount
  await poll.start({ immediate: false });
  assert.equal(timers.length, 1);
  timers.shift()();
  await poll.refreshNow(); // the scheduled poll in flight
  Object.assign(filters, { folder: "trash", accountId: "7", unread: true, hasOtp: true });
  await state.loadList({ keep: false }); // filter change
  await state.loadList(); // the 刷新 button
  poll.stop(); state.dispose();

  assert.ok(requests.length >= 4);
  assert.deepEqual(requests.filter((request) => !request.startsWith("GET ")), []);
  assert.deepEqual(requests.filter((request) => /\/accounts\/[^/]+\/sync/.test(request)), []);
});

function receiveFixture(t, { failSync = new Set() } = {}) {
  const requests = [];
  t.mock.method(globalThis, "fetch", async (url, options = {}) => {
    const method = options.method || "GET", path = new URL(String(url), "http://local").pathname;
    requests.push(`${method} ${path}`);
    const sync = path.match(/\/accounts\/(\d+)\/sync$/);
    if (method === "POST" && sync) {
      if (failSync.has(sync[1])) return new Response(JSON.stringify({ error: { code: "SYNC_FAILED", message: "同步失败，请检查连接状态" } }), { status: 502, headers: { "content-type": "application/json" } });
      // The backend answers 202 + sync_pending while the request is still queued.
      const data = { account: { id: Number(sync[1]), email: `owner${sync[1]}@icloud.com`, enabled: true, last_sync_status: "pending" }, aliases: [], sync_pending: true };
      return new Response(JSON.stringify({ data }), { status: 202, headers: { "content-type": "application/json" } });
    }
    const data = { items: [{ alias_id: 1, mailbox_uid: 1, subject: "邮件" }], total: 1, unread_count: 1 };
    return new Response(JSON.stringify({ data }), { status: 200, headers: { "content-type": "application/json" } });
  });
  const filters = reactive({ folder: "inbox", query: "", accountId: "", aliasId: "", groupId: "", unread: false, starred: false, hasOtp: false });
  const state = useInboxMessages({ filters, page: ref(1), getMessages, getMessage, patchMessage, batchMessages, getCsrfToken: () => "csrf-fixture", onError: assert.fail });
  const accounts = [{ id: 1, email: "a@icloud.com", enabled: true }, { id: 2, email: "b@icloud.com", enabled: false }, { id: 3, email: "c@icloud.com", enabled: true }];
  let submittedEvents = 0;
  const receiver = useInboxReceive({ getAccounts: () => accounts, getAccountFilter: () => filters.accountId, syncAccount, getCsrfToken: () => "csrf-fixture", reloadList: () => state.loadList({ silent: true }), now: () => new Date(2026, 10, 3, 9, 5), onSubmitted: () => { submittedEvents++; } });
  return { requests, filters, state, receiver, submitted: () => submittedEvents };
}
const syncPosts = (requests) => requests.filter((request) => request.startsWith("POST ") && /\/accounts\/[^/]+\/sync$/.test(request));

test("刷新 never submits receiving; only 收取 posts sync for enabled accounts", async (t) => {
  const { requests, state, receiver, submitted } = receiveFixture(t);
  await state.loadList(); // mount
  await state.loadList(); // the 刷新 button
  assert.deepEqual(syncPosts(requests), []);
  assert.equal(receiver.notice.value, null);

  await receiver.receive(); // the 收取 button
  state.dispose();
  assert.deepEqual(syncPosts(requests), ["POST /admin/api/v1/accounts/1/sync", "POST /admin/api/v1/accounts/3/sync"]);
  assert.equal(submitted(), 1);
  // The list is re-read locally after submission, never via another upstream call.
  assert.equal(requests.at(-1).startsWith("GET "), true);
});

test("收取 reports a 202 sync_pending as submitted, never as finished", async (t) => {
  const { state, receiver } = receiveFixture(t);
  const pending = receiver.receive();
  assert.equal(receiver.receiving.value, true);
  assert.deepEqual(receiver.notice.value, { error: false, pending: true, text: "正在提交收件请求…" });
  await pending;
  state.dispose();
  assert.equal(receiver.receiving.value, false);
  assert.equal(receiver.notice.value.text, "收件请求已提交于 09:05");
  assert.equal(receiver.notice.value.error, false);
  assert.equal(receiver.notice.value.pending, undefined);
  assert.doesNotMatch(receiver.notice.value.text, /完成|检查/);
});

test("收取 follows the account filter and reports accounts that were not submitted", async (t) => {
  const { requests, filters, state, receiver, submitted } = receiveFixture(t, { failSync: new Set(["3"]) });
  filters.accountId = "3";
  await receiver.receive();
  assert.deepEqual(syncPosts(requests), ["POST /admin/api/v1/accounts/3/sync"]);
  assert.equal(submitted(), 0);
  assert.equal(receiver.notice.value.error, true);
  assert.match(receiver.notice.value.text, /^1 个主号未成功提交收件请求：同步失败/);
  assert.deepEqual(receiver.notice.value.to, { name: "account-detail", params: { id: 3 } });

  filters.accountId = "";
  await receiver.receive();
  state.dispose();
  assert.equal(receiver.notice.value.error, true);
  assert.match(receiver.notice.value.text, /^收件请求已提交于 09:05；另有 1 个主号未成功提交：/);

  filters.accountId = "2"; // disabled account: nothing to submit
  const before = syncPosts(requests).length;
  await receiver.receive();
  assert.equal(syncPosts(requests).length, before);
  assert.equal(receiver.notice.value.text, "请先添加并启用一个主号，再收取新邮件。");
});
