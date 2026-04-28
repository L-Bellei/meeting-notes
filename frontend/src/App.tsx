import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { GetPort } from "./wailsjs/go/main/App"
import { initApi } from "./hooks/useApi"
import { usePipeline } from "./hooks/usePipeline"
import { Sidebar } from "./components/layout/Sidebar"
import { MeetingList } from "./components/layout/MeetingList"
import { MeetingDetail } from "./components/layout/MeetingDetail"
import { Toolbar } from "./components/layout/Toolbar"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
})

function AppInner() {
  const [ready, setReady] = useState(false)
  const [selectedThemeId, setSelectedThemeId] = useState<string | null>(null)
  const [selectedMeetingId, setSelectedMeetingId] = useState<string | null>(null)
  const [recordingModalOpen, setRecordingModalOpen] = useState(false)

  useEffect(() => {
    GetPort().then(port => {
      initApi(port)
      setReady(true)
    })
  }, [])

  usePipeline()

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground text-sm">
        Iniciando...
      </div>
    )
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <Toolbar
        onRecord={() => setRecordingModalOpen(true)}
        recordingModalOpen={recordingModalOpen}
        onRecordingModalClose={() => setRecordingModalOpen(false)}
        onMeetingCreated={(id: string) => setSelectedMeetingId(id)}
      />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          selectedThemeId={selectedThemeId}
          onSelectTheme={setSelectedThemeId}
        />
        <MeetingList
          themeId={selectedThemeId}
          selectedMeetingId={selectedMeetingId}
          onSelectMeeting={setSelectedMeetingId}
        />
        <MeetingDetail
          meetingId={selectedMeetingId}
        />
      </div>
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppInner />
    </QueryClientProvider>
  )
}
