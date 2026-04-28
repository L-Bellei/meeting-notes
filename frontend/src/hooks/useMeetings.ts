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
  notes: string | null
  created_at: string
}

export interface MeetingFilters {
  theme_id?: string | null
  status?: string
  q?: string
  started_after?: string
  started_before?: string
}

export function useMeetings(filters: MeetingFilters = {}) {
  const params = new URLSearchParams()
  if (filters.theme_id) params.set("theme_id", filters.theme_id)
  if (filters.status) params.set("status", filters.status)
  if (filters.q) params.set("q", filters.q)
  if (filters.started_after) params.set("started_after", filters.started_after)
  if (filters.started_before) params.set("started_before", filters.started_before)
  const qs = params.toString()

  return useQuery({
    queryKey: ["meetings", filters],
    queryFn: () => api<Meeting[]>(`/api/meetings${qs ? "?" + qs : ""}`),
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
