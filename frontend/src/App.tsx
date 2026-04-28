import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { GetPort } from "./wailsjs/go/main/App"
import { initApi } from "./hooks/useApi"
import { usePipeline } from "./hooks/usePipeline"
import { Sidebar } from "./components/layout/Sidebar"
import { MeetingList } from "./components/layout/MeetingList"
import { MeetingDetail } from "./components/layout/MeetingDetail"
import { Toolbar } from "./components/layout/Toolbar"
import { RecordingModal } from "./components/recording/RecordingModal"
import { SettingsModal } from "./components/settings/SettingsModal"
import { Spinner } from "./components/ui/spinner"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
})

function AppInner() {
  const [ready, setReady] = useState(false)
  const [selectedThemeId, setSelectedThemeId] = useState<string | null>(null)
  const [selectedMeetingId, setSelectedMeetingId] = useState<string | null>(null)
  const [recordingModalOpen, setRecordingModalOpen] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

  useEffect(() => {
    GetPort().then(port => {
      initApi(port)
      setReady(true)
    })
  }, [])

  usePipeline()

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center flex-col gap-3 text-muted-foreground text-sm animate-fade-in">
        <Spinner size={24} className="text-primary" />
        Iniciando...
      </div>
    )
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <Toolbar
        onToggleSidebar={() => setSidebarOpen(o => !o)}
        onRecord={() => setRecordingModalOpen(true)}
        onSettings={() => setSettingsOpen(true)}
      />
      <Sidebar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        selectedThemeId={selectedThemeId}
        onSelectTheme={setSelectedThemeId}
      />
      <div className="flex flex-1 overflow-hidden">
        <MeetingList
          themeId={selectedThemeId}
          selectedMeetingId={selectedMeetingId}
          onSelectMeeting={setSelectedMeetingId}
          onMeetingDeleted={id => { if (selectedMeetingId === id) setSelectedMeetingId(null) }}
        />
        <MeetingDetail
          meetingId={selectedMeetingId}
          onDeleted={() => setSelectedMeetingId(null)}
        />
      </div>
      <RecordingModal
        open={recordingModalOpen}
        onClose={() => setRecordingModalOpen(false)}
        onMeetingCreated={(id: string) => setSelectedMeetingId(id)}
      />
      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
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
