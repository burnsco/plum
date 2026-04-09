/** Returns the final path segment for display (POSIX or Windows separators). */
export function fileNameFromPath(path: string): string {
  const trimmed = path.trim();
  if (trimmed === "") return "";
  const slash = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"));
  return slash >= 0 ? trimmed.slice(slash + 1) : trimmed;
}
