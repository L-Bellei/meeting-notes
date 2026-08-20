import { useSettings, useUpdateSettings } from "./useSettings"

export function useSidebarPinned() {
  const { data: settings } = useSettings()
  const updateSettings = useUpdateSettings()
  const pinned = settings?.sidebar_pinned === "true"

  return {
    pinned,
    toggle: () => updateSettings.mutate({ sidebar_pinned: pinned ? "false" : "true" }),
  }
}
