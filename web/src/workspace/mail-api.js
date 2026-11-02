import { apiRequest } from "../api/client.js";
import { ADMIN_API_PREFIX } from "../utils/runtimePath.js";

export function normalizeMessage(raw = {}) {
  return {
    ...raw,
    id: raw.id ?? raw.ID,
    aliasId: raw.alias_id ?? raw.aliasId ?? "",
    mailboxUid: raw.mailbox_uid ?? raw.mailboxUid ?? "",
    accountId: raw.account_id ?? raw.accountId ?? "",
    address: raw.address ?? "",
    aliasLabel: raw.alias_label ?? raw.aliasLabel ?? raw.address ?? "",
    mailboxNote: raw.mailbox_note ?? raw.mailboxNote ?? "",
    groupName: raw.group_name ?? raw.groupName ?? "",
    subject: raw.subject || "(无主题)",
    from: Array.isArray(raw.from) ? raw.from.map((item) => ({ ...item, address: item.address || item.email || "" })) : [],
    receivedAt: raw.received_at ?? raw.receivedAt ?? "",
    contentState: raw.content_state ?? raw.contentState ?? "metadata_only",
    otp: raw.otp ?? null,
    read: Boolean(raw.read), starred: Boolean(raw.starred),
    archived: Boolean(raw.archived), deleted: Boolean(raw.deleted),
  };
}

export async function getMessages(options = {}) {
  const q = new URLSearchParams();
  for (const [key, value] of Object.entries({ account_id: options.accountId, alias_id: options.aliasId, group_id: options.groupId, query: options.query, folder: options.folder || "inbox", unread: options.unread ? "1" : "", starred: options.starred ? "1" : "", limit: options.limit ?? 50, offset: options.offset ?? 0 })) {
    if (value !== undefined && value !== "") q.set(key, String(value));
  }
  const data = await apiRequest(`/messages?${q}`, { signal: options.signal });
  const items = Array.isArray(data) ? data : data?.items || data?.messages || [];
  return { items: items.map(normalizeMessage), total: Number(data?.total) || items.length, unreadCount: Number(data?.unread_count ?? data?.unreadCount) || 0, hasMore: Boolean(data?.has_more ?? data?.hasMore) };
}

export async function getMessage(aliasId, uid, options = {}) {
  const data = await apiRequest(`/mailboxes/${encodeURIComponent(aliasId)}/messages/${encodeURIComponent(uid)}`, { signal: options.signal });
  const raw = data?.message || data || {};
  return { ...(data || {}), ...normalizeMessage(raw), message: normalizeMessage(raw), text: data?.text ?? raw.text ?? "", html: data?.html ?? raw.html ?? "", attachments: Array.isArray(data?.attachments) ? data.attachments : [], contentAvailable: data?.content_available === true };
}

export function patchMessage(aliasId, uid, patch, csrfToken) {
  return apiRequest(`/mailboxes/${encodeURIComponent(aliasId)}/messages/${encodeURIComponent(uid)}`, { method: "PATCH", body: patch, csrfToken }).then(normalizeMessage);
}

export function messagePath(path) {
  if (!path) return "";
  const value = String(path);
  if (!/^\/mailboxes\/[1-9]\d*\/messages\/[1-9]\d*\/(source|attachments\/[1-9]\d*)$/.test(value)) return "";
  return `${ADMIN_API_PREFIX}${value}`;
}
