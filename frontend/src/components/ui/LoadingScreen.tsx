import iconUrl from "../../assets/iconv2.png"
import { cn } from "../../lib/utils"
import { Spinner } from "./spinner"

export type CheckStatus = "hidden" | "pending" | "loading" | "done" | "error"

export interface LoadingCheck {
  label: string
  status: CheckStatus
  error?: string
}

interface LoadingScreenProps {
  checks: LoadingCheck[]
  fading: boolean
}

export function LoadingScreen({ checks, fading }: LoadingScreenProps) {
  return (
    <div
      className={cn(
        "fixed inset-0 z-50 flex flex-col items-center justify-center gap-7 bg-background transition-opacity duration-300",
        fading ? "opacity-0" : "opacity-100"
      )}
    >
      <div className="flex flex-col items-center gap-3">
        <img src={iconUrl} width={80} height={80} className="rounded-2xl" alt="Meeting Notes" />
        <span className="text-lg font-semibold tracking-tight">Meeting Notes</span>
      </div>

      <Spinner size={28} className="text-primary" />

      <div className="flex flex-col gap-3 min-w-[220px]">
        {checks.filter(c => c.status !== "hidden").map(check => (
          <div key={check.label} className="flex flex-col gap-1">
            <div className="flex items-center gap-2.5">
              <CheckIcon status={check.status} />
              <span className={cn("text-[13px]", labelClass(check.status))}>
                {check.label}
              </span>
            </div>
            {check.status === "error" && check.error && (
              <p className="ml-7 text-[11px] text-destructive">{check.error}</p>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function CheckIcon({ status }: { status: CheckStatus }) {
  if (status === "done") {
    return (
      <div className="flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center rounded-full bg-green-500">
        <svg width="10" height="10" viewBox="0 0 12 12">
          <polyline points="2,6 5,9 10,3" stroke="white" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
    )
  }
  if (status === "loading") {
    return (
      <div className="flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center rounded-full border-2 border-primary">
        <Spinner size={10} className="text-primary" />
      </div>
    )
  }
  if (status === "error") {
    return (
      <div className="flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center rounded-full bg-destructive">
        <svg width="10" height="10" viewBox="0 0 12 12">
          <line x1="3" y1="3" x2="9" y2="9" stroke="white" strokeWidth="2" strokeLinecap="round" />
          <line x1="9" y1="3" x2="3" y2="9" stroke="white" strokeWidth="2" strokeLinecap="round" />
        </svg>
      </div>
    )
  }
  // pending
  return <div className="h-[18px] w-[18px] flex-shrink-0 rounded-full border-2 border-muted opacity-35" />
}

function labelClass(status: CheckStatus): string {
  switch (status) {
    case "done":    return "text-green-400"
    case "loading": return "text-primary/80"
    case "error":   return "text-destructive"
    default:        return "text-muted-foreground opacity-50"
  }
}
