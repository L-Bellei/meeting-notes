import { cn } from "../../lib/utils"

export function Spinner({ size = 16, className }: { size?: number; className?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      className={cn("animate-spin", className)}
    >
      <circle
        cx="12" cy="12" r="10"
        stroke="currentColor"
        strokeWidth="3"
        fill="none"
        strokeDasharray="40 20"
        strokeLinecap="round"
      />
    </svg>
  )
}
