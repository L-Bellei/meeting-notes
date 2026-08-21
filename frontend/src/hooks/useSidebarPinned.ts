import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useSettings, type Settings } from "./useSettings"
import { api } from "./useApi"

function useSetSidebarPinned() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (pinned: boolean) =>
      api<Settings>("/api/settings", {
        method: "PUT",
        body: JSON.stringify({ sidebar_pinned: pinned ? "true" : "false" }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["settings"] }),
  })
}

export function useSidebarPinned() {
  const { data: settings } = useSettings()
  const setPinned = useSetSidebarPinned()
  const pinned = settings?.sidebar_pinned === "true"

  return {
    pinned,
    toggle: () => setPinned.mutate(!pinned),
  }
}
