/** Paths under /tasks that should not highlight the home/tasks nav item. */
const TASKS_HOME_EXCLUSIONS = ["/tasks/ongoing"];

/**
 * Whether a nav href should appear active for the current pathname.
 * Feature PRs add routes to TASKS_HOME_EXCLUSIONS when they introduce
 * sibling task views (e.g. ongoing dashboard).
 */
export function isNavSectionActive(href: string, pathname: string): boolean {
  if (pathname === href) {
    return true;
  }

  if (href === "/") {
    if (pathname === "/tasks") {
      return true;
    }

    if (pathname.startsWith("/tasks/")) {
      return !TASKS_HOME_EXCLUSIONS.includes(pathname);
    }
  }

  return false;
}
