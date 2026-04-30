import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { GetPort } from "./wailsjs/go/main/App"
import { EventsOn } from "./wailsjs/runtime/runtime"
import { initApi } from "./hooks/useApi"
import { usePipeline } from "./hooks/usePipeline"
import { Sidebar } from "./components/layout/Sidebar"
import { MeetingList } from "./components/layout/MeetingList"
import { MeetingDetail } from "./components/layout/MeetingDetail"
import { Toolbar } from "./components/layout/Toolbar"
import { RecordingModal } from "./components/recording/RecordingModal"
import { SettingsModal } from "./components/settings/SettingsModal"
import { Spinner } from "./components/ui/spinner"
import { BoardView } from "./components/board/BoardView"
import { SearchModal } from "./components/search/SearchModal"

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
  const [activeView, setActiveView] = useState<"meetings" | "board">("meetings")
  const [searchOpen, setSearchOpen] = useState(false)
  const [highlightQuery, setHighlightQuery] = useState<string | undefined>(undefined)

  useEffect(() => {
    GetPort().then(port => {
      initApi(port)
      setReady(true)
    })
  }, [])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "k" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault()
        setSearchOpen(true)
      }
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [])

  useEffect(() => {
    const unlisten = EventsOn("hotkey:recording-started", ({ meetingId }: { meetingId: string }) => {
      setSelectedMeetingId(meetingId)
      setHighlightQuery(undefined)
      setActiveView("meetings")
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [])

  usePipeline()

  function handleSearchSelect(meetingId: string, query: string) {
    setSelectedMeetingId(meetingId)
    setHighlightQuery(query)
    setActiveView("meetings")
  }

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
        activeView={activeView}
        onChangeView={setActiveView}
      />
      <Sidebar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        selectedThemeId={selectedThemeId}
        onSelectTheme={setSelectedThemeId}
      />
      <div className="flex flex-1 overflow-hidden">
        {activeView === "board" ? (
          <BoardView />
        ) : (
          <>
            <MeetingList
              themeId={selectedThemeId}
              selectedMeetingId={selectedMeetingId}
              onSelectMeeting={id => { setSelectedMeetingId(id); setHighlightQuery(undefined) }}
              onMeetingDeleted={id => { if (selectedMeetingId === id) setSelectedMeetingId(null) }}
              onOpenSearch={() => setSearchOpen(true)}
            />
            <MeetingDetail
              meetingId={selectedMeetingId}
              onDeleted={() => setSelectedMeetingId(null)}
              highlightQuery={highlightQuery}
            />
          </>
        )}
      </div>
      {searchOpen && (
        <SearchModal
          onClose={() => setSearchOpen(false)}
          onSelect={handleSearchSelect}
        />
      )}
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
