import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "../../lib/utils"

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors",
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary/20 text-purple-400",
        secondary: "border-transparent bg-muted text-muted-foreground",
        destructive: "border-transparent bg-destructive/20 text-red-400",
        outline: "border-border text-foreground",
        purple: "bg-purple-600/20 text-purple-400 border-purple-500/30",
        recording: "border-transparent bg-red-500/20 text-red-400",
        transcribing: "border-transparent bg-yellow-500/20 text-yellow-400",
        processing: "border-transparent bg-yellow-400/20 text-yellow-400",
        completed: "border-transparent bg-green-500/20 text-green-500",
        failed: "border-transparent bg-red-700/20 text-red-400",
        pending: "border-transparent bg-gray-400/20 text-gray-400",
      },
    },
    defaultVariants: { variant: "default" },
  }
)

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}
