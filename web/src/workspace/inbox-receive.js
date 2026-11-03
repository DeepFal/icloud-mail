import { ref } from "vue";

const clock = (at) => new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false }).format(at);

// The inbox's only path to upstream receiving: the explicit 「收取」 button.
// Mount, polling, filters and 「刷新」 must stay on local list reads.
// A 200 and a 202/sync_pending both only mean the request was submitted, so
// the notice never claims the upstream check has finished.
export function useInboxReceive({ getAccounts, getAccountFilter = () => "", syncAccount, getCsrfToken, reloadList, isDisposed = () => false, now = () => new Date(), onSubmitted = () => {} }) {
  const receiving = ref(false), notice = ref(null);
  async function receive() {
    if (receiving.value) return;
    const filter = getAccountFilter();
    const targets = getAccounts().filter((account) => account.enabled && (!filter || String(account.id) === filter));
    if (!targets.length) { notice.value = { error: true, text: "请先添加并启用一个主号，再收取新邮件。", to: { name: "accounts" }, link: "设置主号" }; return; }
    receiving.value = true;
    notice.value = { error: false, pending: true, text: "正在提交收件请求…" };
    let submitted = 0;
    const failed = [];
    try {
      for (const account of targets) {
        if (isDisposed()) return;
        try { await syncAccount(account.id, getCsrfToken()); submitted++; }
        catch (cause) { failed.push({ account, message: cause?.message || "提交结果未确认" }); }
      }
      if (isDisposed()) return;
      const first = failed[0], at = clock(now());
      if (!first) {
        notice.value = { error: false, text: `收件请求已提交于 ${at}`, to: targets.length === 1 ? { name: "account-detail", params: { id: targets[0].id } } : { name: "accounts" }, link: "查看收件状态" };
      } else {
        const prefix = submitted ? `收件请求已提交于 ${at}；另有 ${failed.length} 个主号未成功提交` : `${failed.length} 个主号未成功提交收件请求`;
        notice.value = { error: true, text: `${prefix}：${first.message} 请先查看状态再重试。`, to: { name: "account-detail", params: { id: first.account.id } }, link: "查看主号状态" };
      }
      if (submitted) onSubmitted();
      await reloadList();
    } finally { if (!isDisposed()) receiving.value = false; }
  }
  return { receiving, notice, receive };
}
