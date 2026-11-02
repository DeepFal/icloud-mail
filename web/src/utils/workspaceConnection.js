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

export function connectionTarget(accounts = [], loaded = false, error = false) {
  if (error || !loaded) return { name: "accounts" };
  const summary = summarizeConnections(accounts, loaded, error);
  if (summary.kind === "noaccount") return { name: "account-new" };
  const actionable = accounts.filter((account) => {
    const state = classifyAccountConnection(account);
    return state === "failed" || state === "loading";
  });
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
  if (states.includes("failed")) return { kind: "failed", label: "有主号需要处理" };
  if (states.includes("loading")) return { kind: "loading", label: accounts.some(isReceiving) ? "正在收取邮件" : "等待首次收件验证" };
  if (states.every((state) => state === "disabled")) return { kind: "disabled", label: "主号收件已停用" };
  if (states.includes("disabled")) return { kind: "disabled", label: "有主号已停用" };
  return { kind: "ready", label: "最近收件同步成功" };
}
