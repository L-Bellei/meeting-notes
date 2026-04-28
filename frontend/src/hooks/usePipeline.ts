import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { EventsOn } from "../wailsjs/runtime/runtime"

interface PipelinePayload {
  meeting_id: string
  status: string
}

export function usePipeline() {
  const qc = useQueryClient()
  useEffect(() => {
    const unlisten = EventsOn("pipeline:status", (payload: PipelinePayload) => {
      qc.invalidateQueries({ queryKey: ["meeting", payload.meeting_id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [qc])
}
