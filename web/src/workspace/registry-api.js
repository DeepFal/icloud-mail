import { apiRequest } from "../api/client.js";

function queryString(values) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== "" && value !== false) query.set(key, String(value));
  }
  return query.toString();
}
export const getServices = ({ query = "", includeArchived = false, signal } = {}) => apiRequest(`/services?${queryString({ query, include_archived: includeArchived ? 1 : "" })}`, { signal }).then((result) => result.items || []);
export const createService = (body, csrfToken) => apiRequest("/services", { method: "POST", body, csrfToken });
export const updateService = (id, body, csrfToken) => apiRequest(`/services/${encodeURIComponent(id)}`, { method: "PATCH", body, csrfToken });
export const getMailboxProfile = (id, { signal } = {}) => apiRequest(`/aliases/${encodeURIComponent(id)}/profile`, { signal });
export const saveMailboxNote = (id, note, csrfToken) => apiRequest(`/aliases/${encodeURIComponent(id)}/profile`, { method: "PATCH", body: { note }, csrfToken });
export const saveRegistration = (id, body, csrfToken) => apiRequest(`/aliases/${encodeURIComponent(id)}/registrations`, { method: "PUT", body, csrfToken });
export const removeRegistration = (aliasId, id, csrfToken) => apiRequest(`/aliases/${encodeURIComponent(aliasId)}/registrations/${encodeURIComponent(id)}`, { method: "DELETE", csrfToken });
export const batchSaveRegistrations = (body, csrfToken) => apiRequest("/service-registrations/batch", { method: "POST", body, csrfToken });
export const getServiceMailboxes = ({ serviceId, query = "", status = "", accountId = "", groupId = "", enabledOnly = false, limit = 50, offset = 0, signal } = {}) => apiRequest(`/service-mailboxes?${queryString({ service_id: serviceId, query, status, account_id: accountId, group_id: groupId, enabled_only: enabledOnly ? 1 : "", limit, offset })}`, { signal });

export const registrationStates = [
  { value: "registered", label: "已注册" },
  { value: "pending", label: "待确认" },
  { value: "not_registered", label: "确认未注册" },
  { value: "closed", label: "已停用" },
];
export const registrationLabel = (status) => registrationStates.find((item) => item.value === status)?.label || "未记录";

// Literal substring matching across human-readable fields. No regex or fuzzy
// rank can turn a different mailbox into an apparently exact address match.
export function matchesMailboxOption(option, query) {
  const text = String(query || "").trim().toLocaleLowerCase();
  return !text || [option.label, option.note, option.secondary].some((value) => String(value || "").toLocaleLowerCase().includes(text));
}
