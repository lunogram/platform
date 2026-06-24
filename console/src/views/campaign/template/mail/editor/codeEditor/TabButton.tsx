import { cn } from "@/utils"

interface TabButtonProps {
    active: boolean
    onClick: () => void
    icon: React.ReactNode
    label: string
}

export function TabButton({ active, onClick, icon, label }: TabButtonProps) {
    return (
        <button
            onClick={onClick}
            className={cn(
                "flex items-center gap-2 px-4 py-2.5 text-sm font-medium transition-all duration-200 cursor-pointer",
                "border-b-2 -mb-px",
                active
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground hover:border-muted-foreground/30",
            )}
        >
            {icon}
            <span>{label}</span>
        </button>
    )
}

interface PreviewTabProps {
    icon: React.ReactNode
    label: string
}

export function PreviewTab({ icon, label }: PreviewTabProps) {
    return (
        <div
            className={cn(
                "flex items-center gap-2 px-4 py-2.5 text-sm font-medium",
                "border-b-2 -mb-px border-primary text-foreground",
            )}
        >
            {icon}
            <span>{label}</span>
        </div>
    )
}
