import { translate, type Locale } from "$lib/i18n";

/**
 * A user we can resolve to a real display name, indexed by subject id.
 * Used to surface names for overseers other than the current user.
 */
export interface KnownOverseer {
  sub: string;
  name?: string;
  email?: string;
}

/**
 * Resolve an overseer user id to a human display label.
 *
 * Overseers are persisted as raw subject ids (`overseerUserId`). Showing
 * those ids to users is never acceptable, so every "linked things" surface
 * (binding matrix, overseer panel, configuration summary) funnels through
 * here instead of rendering the id — or the locale-agnostic "You" the
 * backend emits — directly.
 *
 * Resolution rules:
 * - empty id → "" (the caller picks its own placeholder, e.g. "Unassigned")
 * - current user (`id === sessionUserSub`) → localized "You" / "Eu"
 * - a known user supplied via `knownUsers` → their name (or email)
 * - anyone else → localized generic fallback ("Someone" / "Alguém")
 *
 * The current user is matched by id equality only — never by string-matching
 * a label — so the backend's fallback "You" string is never trusted.
 */
export function localizedOverseerLabel(
  userId: string,
  sessionUserSub: string | undefined,
  locale: Locale,
  knownUsers: Map<string, KnownOverseer> = new Map(),
): string {
  const id = (userId ?? "").trim();
  if (!id) return "";
  if (sessionUserSub && id === sessionUserSub) {
    return translate("assistant.overseer.you", locale);
  }
  const known = knownUsers.get(id);
  const name = known?.name?.trim();
  if (name) return name;
  const email = known?.email?.trim();
  if (email) return email;
  return translate("assistant.overseer.someone", locale);
}

/**
 * Single uppercase glyph for an overseer avatar chip. Falls back to a bullet
 * when there is nothing to seed the initial from (e.g. an unassigned slot).
 */
export function overseerAvatarInitial(label: string): string {
  const trimmed = (label ?? "").trim();
  return trimmed ? trimmed.charAt(0).toUpperCase() : "•";
}
