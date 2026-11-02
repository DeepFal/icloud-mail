import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const source = await readFile(new URL("../src/views/AccountDetailView.vue", import.meta.url), "utf8");

test("directory sync reports its scope without a descriptive page heading", () => {
  assert.match(source, /同步地址目录/);
  assert.match(source, /本次未读取邮件/);
  assert.doesNotMatch(source, /title="IMAP 邮件同步"/);
  assert.match(source, /title="邮件接收"/);
  assert.match(source, /<ReceivingSettings/);
  for (const header of source.match(/<SectionHeader\b[^>]*>/g) || []) assert.doesNotMatch(header, /description=/);
});

test("directory sync shows removal results and incomplete reconciliation clearly", () => {
  assert.match(source, /aliasSyncSummary\.removedCount/);
  assert.match(source, /清理 \{\{ aliasSyncSummary\.removedCount \}\} 个/);
  assert.match(source, /部分目录核对未完成，未清理缺失地址/);
  assert.doesNotMatch(source, /appleDirectory|快照时间|目录缺项保留本地记录/);
  assert.doesNotMatch(source, /aliasCount: resolvedTotal/);
});

test("manual whole-account fetch is advanced and authentication failures block it", () => {
  const details = source.match(/<details class="settings-disclosure">([\s\S]*?)<\/details>/)?.[1];
  assert.ok(details);
  assert.match(details, /手动同步主号邮件/);
  assert.match(details, /:disabled="[^"]*isIMAPAuthenticationFailure\(account.lastSyncError\)"/);
  assert.match(source, /async function syncNow\(\) \{\s*if \([^\n]*isIMAPAuthenticationFailure\(account.value.lastSyncError\)\) return/);
  assert.match(source, /配置收件凭据/);
  assert.match(source, /地址目录同步不代表收件恢复/);
});
