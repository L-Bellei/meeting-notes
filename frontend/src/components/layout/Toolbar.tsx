import { Mic } from "lucide-react"
import { Button } from "../ui/button"
import { RecordingModal } from "../recording/RecordingModal"

interface ToolbarProps {
  onRecord: () => void
  recordingModalOpen: boolean
  onRecordingModalClose: () => void
  onMeetingCreated: (id: string) => void
}

export function Toolbar({ onRecord, recordingModalOpen, onRecordingModalClose, onMeetingCreated }: ToolbarProps) {
  return (
    <div className="h-12 border-b flex items-center px-4 gap-3 flex-shrink-0">
      <span className="font-semibold text-sm text-foreground">Meeting Notes</span>
      <div className="flex-1" />
      <Button size="sm" onClick={onRecord}>
        <Mic size={14} className="mr-1.5" /> Gravar Nova Reunião
      </Button>
      <RecordingModal
        open={recordingModalOpen}
        onClose={onRecordingModalClose}
        onMeetingCreated={onMeetingCreated}
      />
    </div>
  )
}
