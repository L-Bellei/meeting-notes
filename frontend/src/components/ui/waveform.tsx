import { cn } from "../../lib/utils"

export function Waveform({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-end gap-[2px] h-4", className)}>
      <span className="w-[3px] bg-current rounded-full animate-wave1" style={{ height: "100%" }} />
      <span className="w-[3px] bg-current rounded-full animate-wave2" style={{ height: "100%" }} />
      <span className="w-[3px] bg-current rounded-full animate-wave3" style={{ height: "100%" }} />
      <span className="w-[3px] bg-current rounded-full animate-wave4" style={{ height: "100%" }} />
      <span className="w-[3px] bg-current rounded-full animate-wave5" style={{ height: "100%" }} />
    </span>
  )
}
