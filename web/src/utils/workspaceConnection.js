const SUCCESS = new Set(["ready", "success", "ok", "completed"]);
const NON_FAILURE = new Set(["", "pending", "running", "syncing", "queued"]);
const RECEIVING = new Set(["running", "syncing", "queued"]);
const isReceiving = (account) => account.syncProgress?.active === true || RECEIVING.has(String(account.lastSyncStatus || "").trim().toLowerCase());

export function classifyAccountConnection(account = {}) {
  if (account.enabled === false) return "disabled";
  if (isReceiving(account)) return "loading";
  const status = String(account.lastSyncStatus || "").trim().toLowerCase();
  if (SUCCESS.has(status)) return "ready";
  if (NON_FAILURE.has(status)) return account.lastSyncedAt ? "ready" : "loading";
  return "failed";
}

// Reasons an enabled account needs the operator: receiving failed, an Apple
// session expired, or automatic keep-alive gave up. Never-connected Apple is not one.
export function accountAttentionReasons(account = {}) {
  if (account.enabled === false) return [];
  const reasons = [];
  if (classifyAccountConnection(account) === "failed") reasons.push("收件异常");
  const apple = account.apple;
  if (apple?.web === "expired" || apple?.account === "expired") reasons.push("Apple 会话失效");
  if (apple?.keepAlive === "needs_action") reasons.push("保活待处理");
  return reasons;
}

export const accountNeedsAttention = (account) => accountAttentionReasons(account).length > 0;

export function connectionTarget(accounts = [], loaded = false, error = false) {
  if (error || !loaded) return { name: "accounts" };
  const summary = summarizeConnections(accounts, loaded, error);
  if (summary.kind === "noaccount") return { name: "account-new" };
  const attention = accounts.filter(accountNeedsAttention);
  if (attention.length > 1) return { name: "accounts", query: { attention: "1" } };
  const actionable = attention.length ? attention : accounts.filter((account) => classifyAccountConnection(account) === "loading");
  if (actionable.length === 1 && actionable[0].id != null) {
    return { name: "account-detail", params: { id: String(actionable[0].id) } };
  }
  return { name: "accounts" };
}

export function summarizeConnections(accounts = [], loaded = false, error = false) {
  if (error) return { kind: "failed", label: "收件状态暂未读取" };
  if (!loaded) return { kind: "loading", label: "正在读取主号状态" };
  if (!accounts.length) return { kind: "noaccount", label: "尚未添加主号" };
  const states = accounts.map(classifyAccountConnection);
  const attention = accounts.filter(accountNeedsAttention).length;
  if (attention) return { kind: "failed", label: `${attention} 个主号需要处理` };
  if (states.includes("loading")) return { kind: "loading", label: accounts.some(isReceiving) ? "正在收取邮件" : "等待首次收件验证" };
  if (states.every((state) => state === "disabled")) return { kind: "disabled", label: "主号收件已停用" };
  if (states.includes("disabled")) return { kind: "disabled", label: "有主号已停用" };
  return { kind: "ready", label: "最近收件同步成功" };
}

const CHANNEL_LABELS = {
  healthy: ["正常", "success"], retry_wait: ["重试中", "warning"], expired: ["已失效", "danger"],
  not_connected: ["未连接", "info"], unknown: ["未知", "info"],
};
const KEEP_ALIVE_LABELS = {
  needs_action: ["待处理", "danger"], retry_wait: ["重试中", "warning"], logging_in: ["登录中", "primary"],
  waiting_code: ["登录中", "primary"], authenticated: ["正常", "success"], idle: ["待命", "info"],
};

// Tags for the account list's "Apple 连接" column; empty when there is no Apple state.
export function appleConnectionTags(apple) {
  if (!apple) return [];
  const tags = [["网页", CHANNEL_LABELS[apple.web]], ["账户", CHANNEL_LABELS[apple.account]], ["保活", KEEP_ALIVE_LABELS[apple.keepAlive]]];
  return tags.filter(([, entry]) => entry).map(([name, [label, type]]) => ({ name, label, type }));
}
