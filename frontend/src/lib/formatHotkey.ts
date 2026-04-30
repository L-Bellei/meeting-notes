export function formatHotkey(raw: string): string {
  return raw
    .split("+")
    .map(p => p.charAt(0).toUpperCase() + p.slice(1))
    .join("+")
}
