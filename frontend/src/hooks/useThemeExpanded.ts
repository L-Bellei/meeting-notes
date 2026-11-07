import { useCallback, useEffect, useMemo, useState } from "react"

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
  const activeIds = useMemo(
    () => themeIds.length === 0 ? ids : ids.filter(id => themeIds.includes(id)),
    [ids, themeIds],
  )

  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify(activeIds))
    } catch { /* modo privado / cota — expansão volta a ser efêmera */ }
  }, [activeIds])

  const toggle = useCallback((id: string) => {
    setIds(prev => {
      const current = themeIds.length === 0 ? prev : prev.filter(value => themeIds.includes(value))
      return current.includes(id) ? current.filter(value => value !== id) : [...current, id]
    })
  }, [themeIds])

  const expand = useCallback((id: string) => {
    setIds(prev => {
      const current = themeIds.length === 0 ? prev : prev.filter(value => themeIds.includes(value))
      return current.includes(id) ? current : [...current, id]
    })
  }, [themeIds])

  const expanded: Record<string, boolean> = {}
  for (const id of activeIds) expanded[id] = true

  return { expanded, toggle, expand }
}
