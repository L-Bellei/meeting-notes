import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface Meeting {
  id: string
  theme_id: string | null
  title: string
  started_at: string | null
  duration_seconds: number | null
  status: "pending" | "recording" | "transcribing" | "processing" | "completed" | "failed"
  transcript: string | null
  created_at: string
}

export function useMeetings(themeId?: string | null) {
  return useQuery({
    queryKey: ["meetings", themeId ?? "all"],
    queryFn: () => api<Meeting[]>("/api/meetings"),
    select: (data) => themeId ? data.filter(m => m.theme_id === themeId) : data,
  })
}

export function useCreateMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { title: string; theme_id?: string }) =>
      api<Meeting>("/api/meetings", { method: "POST", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meetings"] }),
  })
}

export function useDeleteMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api<void>(`/api/meetings/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meetings"] }),
  })
}
