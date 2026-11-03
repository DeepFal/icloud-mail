import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { reactive, ref } from "vue";
import { createLiveRefresh } from "../src/utils/liveRefresh.js";
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

test("inbox refresh, mount, polling and filters only reload the local list", async () => {
  const source = await readFile(inboxViewPath, "utf8");
  assert.equal((source.match(/<Refresh :class/g) || []).length, 1);
  assert.match(source, /class="list-refresh" :disabled="loading" @click="loadList\(\)"/);
  assert.match(source, /createLiveRefresh\(\(\) => loadList\(\{ silent: true \}\)\)/);
  assert.match(source, /onMounted\(\(\) => \{ loadMeta\(\); loadList\(\); liveRefresh\.start\(/);
  // The inbox has no path that submits upstream receiving.
  assert.doesNotMatch(source, /syncAccount|\/sync\b|checkUpstream|上游检查|method:/);
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
