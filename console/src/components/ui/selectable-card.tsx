import { Check } from "lucide-react"
import { cn } from "@/utils"

export interface SelectableCardProps {
    title: string
    summary?: string
    active: boolean
    disabled?: boolean
    icon?: React.ReactNode
    onClick: () => void
}

// SelectableCard is the single "pick one" affordance used across the console for
// presets, type pickers, and option groups. The selected treatment is calm and
// monochrome per DESIGN.md — an Ink border, a Surface-Muted fill, and an Ink
// check. No brand color: color stays rationed for status, never selection.
export function SelectableCard({
    title,
    summary,
    active,
    disabled,
    icon,
    onClick,
}: SelectableCardProps) {
    return (
        <button
            type="button"
            onClick={onClick}
            disabled={disabled}
            aria-pressed={active}
            className={cn(
                "relative flex flex-col gap-1 rounded-md border p-3 pr-8 text-left transition-colors",
                "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
                disabled && "cursor-not-allowed opacity-50",
                active
                    ? "border-primary bg-surface-muted"
                    : !disabled && "border-border hover:border-border-strong hover:bg-surface-soft",
            )}
        >
            <span className="flex items-center gap-2 text-sm font-medium leading-none">
                {icon && (
                    <span className={cn("text-ink-soft", active && "text-foreground")}>{icon}</span>
                )}
                {title}
            </span>
            {summary && <span className="text-xs leading-snug text-ink-soft">{summary}</span>}
            {active && <Check className="absolute right-2.5 top-2.5 h-4 w-4 text-foreground" />}
        </button>
    )
}
