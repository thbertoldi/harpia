/**
 * Whether a nav href should appear active for the current pathname.
 */
export function isNavSectionActive(href: string, pathname: string): boolean {
  if (pathname === href) {
    return true;
  }

  return false;
}
