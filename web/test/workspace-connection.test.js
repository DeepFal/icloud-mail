import test from "node:test";
import assert from "node:assert/strict";
import { classifyAccountConnection, connectionTarget, summarizeConnections } from "../src/utils/workspaceConnection.js";

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
  assert.deepEqual(connectionTarget([{ id: 3, lastSyncStatus: "error" }, { id: 4, lastSyncStatus: "error" }], true), { name: "accounts" });
  assert.deepEqual(connectionTarget([{ id: 3, enabled: false }], true), { name: "accounts" });
});
