import { useEffect, useState } from "react"
import { EventsOn } from "../wailsjs/runtime/runtime"

// useAudioStatus tracks whether the audio service is reachable. It starts
// optimistic and flips on the backend's "audio:down"/"audio:ready" transition
// events (emitted by monitorAudioHealth), so a mid-session drop surfaces in the UI.
export function useAudioStatus() {
  const [audioReady, setAudioReady] = useState(true)

  useEffect(() => {
    const offReady = EventsOn("audio:ready", () => setAudioReady(true))
    const offDown = EventsOn("audio:down", () => setAudioReady(false))
    return () => {
      if (typeof offReady === "function") offReady()
      if (typeof offDown === "function") offDown()
    }
  }, [])

  return audioReady
}
