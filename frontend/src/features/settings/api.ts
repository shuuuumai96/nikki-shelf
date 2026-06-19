import { request } from "../../shared/api/client";
import type { AuditEventList } from "./types";

export function listAuditEvents(limit = 100): Promise<AuditEventList> {
  const params = new URLSearchParams({ limit: String(limit) });
  return request<AuditEventList>(`/api/audit/events?${params.toString()}`);
}
