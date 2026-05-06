import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"
import type { Meeting } from "./useMeetings"

export interface Summary {
  id: string
  meeting_id: string
  content: string
  model_used: string
  input_tokens: number
  output_tokens: number
  created_at: string
}

export interface KeyPoint {
  id: string
  meeting_id: string
  position: number
  content: string
}

export interface Task {
  id: string
  meeting_id: string
  description: string
  assignee: string | null
  due_date: string | null
  priority: "low" | "medium" | "high"
  completed: boolean
  created_at: string
}

export interface MeetingDetail extends Meeting {
  summary: Summary | null
  key_points: KeyPoint[]
  tasks: Task[]
}

export function useMeeting(id: string | null) {
  return useQuery({
    queryKey: ["meeting", id],
    queryFn: () => api<MeetingDetail>(`/api/meetings/${id}`),
    enabled: !!id,
  })
}

export function useUpdateMeeting(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<Meeting>) =>
      api<Meeting>(`/api/meetings/${id}`, { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["meeting", id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}

export function useStartRecording(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<void>(`/api/meetings/${id}/start`, { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["meeting", id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}

export function useStopRecording(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (keepAudio: boolean) =>
      api<void>(`/api/meetings/${id}/stop`, {
        method: "POST",
        body: JSON.stringify({ keep_audio: keepAudio }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["meeting", id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}

export function useReprocess(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<void>(`/api/meetings/${id}/process`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", id] }),
  })
}

export function useGenerateSummary(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<Summary>(`/api/meetings/${meetingId}/summary/generate`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useGenerateKeyPoints(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<KeyPoint[]>(`/api/meetings/${meetingId}/key_points/generate`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useGenerateTasks(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<Task[]>(`/api/meetings/${meetingId}/tasks/generate`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useUpdateTask(meetingId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<Task>) =>
      api<Task>(`/api/meetings/${meetingId}/tasks/${taskId}`, { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useRetranscribe(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api(`/api/meetings/${meetingId}/retranscribe`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}
