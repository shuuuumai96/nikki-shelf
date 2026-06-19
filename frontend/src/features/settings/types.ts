export type AuditOutcome = "succeeded" | "failed";

export type AuditEvent = {
  id: number;
  eventType: string;
  outcome: AuditOutcome;
  actorUserId?: number;
  actorUsername?: string;
  actorRole?: "owner" | "user" | string;
  targetType?: string;
  targetId?: string;
  reasonKind?: string;
  requestId?: string;
  remoteIp?: string;
  metadata?: Record<string, string>;
  createdAt: string;
};

export type AuditEventList = {
  items: AuditEvent[];
};
