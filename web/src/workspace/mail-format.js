// Mail from the current year shows month/day and time; older mail shows its full date.
export function formatMailDate(value, now = new Date()) {
  const at = new Date(value);
  if (!Number.isFinite(at.getTime())) return "时间未知";
  if (at.getFullYear() !== now.getFullYear()) {
    const pad = (n) => String(n).padStart(2, "0");
    return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
  }
  return new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(at);
}
