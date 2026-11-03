import test from "node:test";
import assert from "node:assert/strict";
import { accountAttentionReasons, accountNeedsAttention, appleConnectionTags, classifyAccountConnection, connectionTarget, summarizeConnections } from "../src/utils/workspaceConnection.js";

test("workspace connection summarizes loading, errors, empty, disabled and partial success", () => {
  assert.equal(summarizeConnections([], true).kind, "noaccount");
  assert.equal(summarizeConnections([], false).kind, "loading");
  assert.equal(summarizeConnections([], true, true).kind, "failed");
  assert.equal(summarizeConnections([{ id: 1, enabled: false }], true).kind, "disabled");
  assert.equal(summarizeConnections([{ id: 1, lastSyncStatus: "pending" }], true).label, "等待首次收件验证");
  assert.equal(summarizeConnections([{ id: 1, lastSyncStatus: "running", lastSyncedAt: "2026-11-02T01:00:00Z" }], true).label, "正在收取邮件");
  assert.equal(summarizeConnections([{ id: 1, lastSyncStatus: "running" }], true).label, "正在收取邮件");
  assert.equal(summarizeConnections([{ id: 1, lastSyncStatus: "error" }], true).kind, "failed");
  assert.equal(summarizeConnections([{ id: 1, lastSyncStatus: "ok" }, { id: 2, lastSyncStatus: "error" }], true).kind, "failed");
  assert.equal(summarizeConnections([{ id: 1, enabled: false }, { id: 2, lastSyncStatus: "ok" }], true).label, "有主号已停用");
});

test("sync progress active is receiving in progress, not a connection failure", () => {
  assert.equal(classifyAccountConnection({ id: 7, syncProgress: { active: true }, lastSyncStatus: "error" }), "loading");
});

test("connection target drills into one actionable account", () => {
  assert.deepEqual(connectionTarget([], true), { name: "account-new" });
  assert.deepEqual(connectionTarget([{ id: 3, lastSyncStatus: "error" }], true), { name: "account-detail", params: { id: "3" } });
  assert.deepEqual(connectionTarget([{ id: 3, lastSyncStatus: "pending" }], true), { name: "account-detail", params: { id: "3" } });
  assert.deepEqual(connectionTarget([{ id: 3, lastSyncStatus: "error" }, { id: 4, lastSyncStatus: "error" }], true), { name: "accounts", query: { attention: "1" } });
  assert.deepEqual(connectionTarget([{ id: 3, enabled: false }], true), { name: "accounts" });
});

const ok = { id: 1, enabled: true, lastSyncStatus: "ok" };
const apple = (patch) => ({ web: "healthy", account: "healthy", keepAlive: "disabled", needsAttention: false, ...patch });

test("attention covers receiving failure, expired Apple sessions and stuck keep-alive, not never-connected", () => {
  assert.deepEqual(accountAttentionReasons(ok), []);
  assert.deepEqual(accountAttentionReasons({ ...ok, apple: apple({ web: "not_connected", account: "not_connected" }) }), []);
  assert.deepEqual(accountAttentionReasons({ ...ok, apple: apple({ account: "retry_wait", keepAlive: "retry_wait" }) }), []);
  assert.deepEqual(accountAttentionReasons({ ...ok, apple: apple({ web: "expired" }) }), ["Apple 会话失效"]);
  assert.deepEqual(accountAttentionReasons({ ...ok, apple: apple({ account: "expired", keepAlive: "needs_action" }) }), ["Apple 会话失效", "保活待处理"]);
  assert.deepEqual(accountAttentionReasons({ ...ok, lastSyncStatus: "error", apple: apple({ keepAlive: "needs_action" }) }), ["收件异常", "保活待处理"]);
  assert.equal(accountNeedsAttention({ ...ok, enabled: false, lastSyncStatus: "error", apple: apple({ web: "expired" }) }), false);
});

test("sidebar counts accounts needing attention and routes to the filtered list", () => {
  const expired = { ...ok, id: 2, apple: apple({ web: "expired" }) };
  const failed = { ...ok, id: 3, lastSyncStatus: "error" };
  assert.deepEqual(summarizeConnections([ok, expired], true), { kind: "failed", label: "1 个主号需要处理" });
  assert.equal(summarizeConnections([ok, expired, failed], true).label, "2 个主号需要处理");
  assert.deepEqual(connectionTarget([ok, expired], true), { name: "account-detail", params: { id: "2" } });
  assert.deepEqual(connectionTarget([ok, expired, failed], true), { name: "accounts", query: { attention: "1" } });
});

test("apple connection tags describe channels and hide disabled keep-alive", () => {
  assert.deepEqual(appleConnectionTags(null), []);
  assert.deepEqual(appleConnectionTags(apple({ account: "expired", keepAlive: "needs_action" })).map((t) => `${t.name}:${t.label}`), ["网页:正常", "账户:已失效", "保活:待处理"]);
  assert.deepEqual(appleConnectionTags(apple({})).map((t) => t.name), ["网页", "账户"]);
});

test("account list normalizes the apple health summary", async () => {
  const { normalizeAccount } = await import("../src/api/admin.js");
  assert.equal(normalizeAccount({ id: 1 }).apple, null);
  assert.deepEqual(normalizeAccount({ id: 1, apple: { web: "expired", account: "healthy", keep_alive: "needs_action", creation_cooldown_until: "2026-11-03T10:00:00Z", needs_attention: true } }).apple, {
    web: "expired", account: "healthy", keepAlive: "needs_action", creationCooldownUntil: "2026-11-03T10:00:00Z", needsAttention: true,
  });
});
