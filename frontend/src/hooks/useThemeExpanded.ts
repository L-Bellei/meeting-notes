import { useCallback, useEffect, useState } from "react"

const KEY = "theme_expanded"

function read(): string[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === "string") : []
  } catch {
    return []
  }
}

export function useThemeExpanded(themeIds: string[]) {
  const [ids, setIds] = useState<string[]>(read)

  useEffect(() => {
    if (themeIds.length === 0) return
    setIds(prev => {
      const pruned = prev.filter(id => themeIds.includes(id))
      return pruned.length === prev.length ? prev : pruned
    })
  }, [themeIds])

  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify(ids))
    } catch { /* modo privado / cota — expansão volta a ser efêmera */ }
  }, [ids])

  const toggle = useCallback((id: string) => {
    setIds(prev => prev.includes(id) ? prev.filter(v => v !== id) : [...prev, id])
  }, [])

  const expand = useCallback((id: string) => {
    setIds(prev => prev.includes(id) ? prev : [...prev, id])
  }, [])

  const expanded: Record<string, boolean> = {}
  for (const id of ids) expanded[id] = true

  return { expanded, toggle, expand }
}
