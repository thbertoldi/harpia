import {
  mockAuditEvents,
  type AuditEvent,
  type AuditEventFilters,
  type AuditEventsPage,
  type AuditPageToken,
  type FeedbackDecision,
} from "$lib/mocks/audit-events";
import type { Locale } from "$lib/i18n";

export type { AuditEvent, AuditEventFilters, AuditEventsPage, AuditPageToken };

const DEFAULT_PAGE_SIZE = 10;
const STORAGE_KEY = "harpia_mock_audit_events";

function canUseStorage(): boolean {
  return (
    typeof window !== "undefined" && typeof window.localStorage !== "undefined"
  );
}

function loadPersistedAuditEvents(): AuditEvent[] | null {
  if (!canUseStorage()) {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as AuditEvent[];
    return Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function persistAuditEvents(events: AuditEvent[]): void {
  if (!canUseStorage()) {
    return;
  }
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(events));
}

const persistedAuditEvents = loadPersistedAuditEvents();
let seededLocale: Locale = "en";
const runtimeAuditEvents: AuditEvent[] = persistedAuditEvents ?? [
  ...mockAuditEvents(seededLocale),
];

function ensureSeedLocale(locale: Locale): void {
  if (persistedAuditEvents || locale === seededLocale) {
    return;
  }
  seededLocale = locale;
  runtimeAuditEvents.length = 0;
  runtimeAuditEvents.push(...mockAuditEvents(locale));
}

type KeysetCursor = { ts: string; eventId: string };

export function encodePageToken(cursor: KeysetCursor): string {
  return btoa(JSON.stringify(cursor));
}

export function decodePageToken(token: AuditPageToken): KeysetCursor | null {
  if (!token) return null;
  try {
    const parsed = JSON.parse(atob(token)) as KeysetCursor;
    if (typeof parsed.ts === "string" && typeof parsed.eventId === "string") {
      return parsed;
    }
    return null;
  } catch {
    return null;
  }
}

/** Compare events for keyset order: (ts DESC, eventId DESC) */
export function compareAuditEvents(a: AuditEvent, b: AuditEvent): number {
  const tsDiff = b.timestamp.localeCompare(a.timestamp);
  if (tsDiff !== 0) return tsDiff;
  return b.eventId.localeCompare(a.eventId);
}

export function sortAuditEvents(events: AuditEvent[]): AuditEvent[] {
  return [...events].sort(compareAuditEvents);
}

export function filterAuditEvents(
  events: AuditEvent[],
  filters: AuditEventFilters,
): AuditEvent[] {
  const taskId = filters.taskId?.trim().toLowerCase();
  const userId = filters.userId?.trim().toLowerCase();
  const agentType = filters.agentType?.trim().toLowerCase();
  const dateFrom = filters.dateFrom ? startOfDay(filters.dateFrom) : null;
  const dateTo = filters.dateTo ? endOfDay(filters.dateTo) : null;

  return events.filter((event) => {
    if (taskId && !event.taskId.toLowerCase().includes(taskId)) return false;

    if (userId) {
      const isHumanMatch =
        event.actor.kind === "human" &&
        (event.actor.id.toLowerCase().includes(userId) ||
          event.actor.displayName.toLowerCase().includes(userId));
      if (!isHumanMatch) return false;
    }

    if (agentType) {
      const agentMatch =
        event.actor.kind === "agent" &&
        event.actor.agentType?.toLowerCase().includes(agentType);
      if (!agentMatch) return false;
    }

    if (filters.decision && event.decision !== filters.decision) return false;

    const ts = new Date(event.timestamp).getTime();
    if (dateFrom && ts < dateFrom.getTime()) return false;
    if (dateTo && ts > dateTo.getTime()) return false;

    return true;
  });
}

function startOfDay(dateStr: string): Date {
  const [y, m, d] = dateStr.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d, 0, 0, 0, 0));
}

function endOfDay(dateStr: string): Date {
  const [y, m, d] = dateStr.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d, 23, 59, 59, 999));
}

function isBeforeCursor(event: AuditEvent, cursor: KeysetCursor): boolean {
  if (event.timestamp < cursor.ts) return true;
  if (event.timestamp > cursor.ts) return false;
  return event.eventId < cursor.eventId;
}

export function paginateAuditEvents(
  events: AuditEvent[],
  pageToken: AuditPageToken,
  pageSize = DEFAULT_PAGE_SIZE,
): AuditEventsPage {
  const sorted = sortAuditEvents(events);
  const cursor = decodePageToken(pageToken);

  let startIndex = 0;
  if (cursor) {
    startIndex = sorted.findIndex((event) => isBeforeCursor(event, cursor));
    if (startIndex === -1) {
      return { events: [], nextPageToken: null };
    }
  }

  const page = sorted.slice(startIndex, startIndex + pageSize);
  const last = page[page.length - 1];
  const hasMore = startIndex + pageSize < sorted.length;

  return {
    events: page,
    nextPageToken:
      hasMore && last
        ? encodePageToken({ ts: last.timestamp, eventId: last.eventId })
        : null,
  };
}

/**
 * Primary data access — swap this implementation when audit APIs land.
 */
export async function getAuditEvents(
  locale: Locale = "en",
  filters: AuditEventFilters = {},
  pageToken: AuditPageToken = null,
  pageSize = DEFAULT_PAGE_SIZE,
): Promise<AuditEventsPage> {
  ensureSeedLocale(locale);
  const filtered = filterAuditEvents(runtimeAuditEvents, filters);
  return paginateAuditEvents(filtered, pageToken, pageSize);
}

export async function getAllFilteredAuditEvents(
  locale: Locale = "en",
  filters: AuditEventFilters = {},
): Promise<AuditEvent[]> {
  ensureSeedLocale(locale);
  return sortAuditEvents(filterAuditEvents(runtimeAuditEvents, filters));
}

export function appendMockAuditEvent(event: AuditEvent): void {
  runtimeAuditEvents.unshift(event);
  persistAuditEvents(runtimeAuditEvents);
}

export function resetMockAuditEvents(locale: Locale = "en"): void {
  seededLocale = locale;
  runtimeAuditEvents.length = 0;
  runtimeAuditEvents.push(...mockAuditEvents(locale));
  persistAuditEvents(runtimeAuditEvents);
}

export function auditEventsToCsv(events: AuditEvent[]): string {
  const headers = [
    "event_id",
    "timestamp",
    "event_type",
    "bounded_context",
    "actor_kind",
    "actor_id",
    "actor_name",
    "agent_type",
    "task_id",
    "decision",
    "trace_id",
    "payload_diff",
  ];

  const rows = events.map((e) =>
    [
      e.eventId,
      e.timestamp,
      e.eventType,
      e.boundedContext,
      e.actor.kind,
      e.actor.id,
      e.actor.displayName,
      e.actor.agentType ?? "",
      e.taskId,
      e.decision ?? "",
      e.traceId,
      JSON.stringify(e.payloadDiff),
    ]
      .map(csvEscape)
      .join(","),
  );

  return [headers.join(","), ...rows].join("\n");
}

function csvEscape(value: string): string {
  if (value.includes(",") || value.includes('"') || value.includes("\n")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

export function auditEventsToJson(events: AuditEvent[]): string {
  return JSON.stringify(events, null, 2);
}

export function downloadTextFile(
  content: string,
  filename: string,
  mimeType: string,
): void {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

export const FEEDBACK_DECISIONS: FeedbackDecision[] = [
  "approve",
  "reject",
  "modify",
  "escalate",
];

export const AGENT_TYPES = [
  "planner",
  "supervisor",
  "worker",
  "deep_research",
  "workflow",
] as const;
