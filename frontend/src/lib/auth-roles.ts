export type HarpiaRole = "Leader" | "Overseer" | "Engineer";

const ENGINEER_ROLE: HarpiaRole = "Engineer";

export function getUserRole(
  user: { role?: string } | null | undefined,
): HarpiaRole {
  if (user?.role === "Engineer" || user?.role === "Overseer") {
    return user.role;
  }
  return "Leader";
}

export function isEngineer(
  user: { role?: string } | null | undefined,
): boolean {
  return getUserRole(user) === ENGINEER_ROLE;
}
