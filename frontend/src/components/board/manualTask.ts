export function parseManualTask(s: string): { text: string; done: boolean } {
  if (s.startsWith("[x] ")) return { text: s.slice(4), done: true }
  if (s.startsWith("[ ] ")) return { text: s.slice(4), done: false }
  return { text: s, done: false }
}

export function encodeManualTask(text: string, done: boolean): string {
  return `${done ? "[x]" : "[ ]"} ${text}`
}
