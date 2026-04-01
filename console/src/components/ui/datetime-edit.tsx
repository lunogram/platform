import { type ReactNode, useState } from "react"
import { Pencil } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

interface DateTimeEditProps {
    /** ISO 8601 datetime string (e.g. "2024-01-15T10:30:00Z") */
    value: string
    /** Called with the new ISO datetime string when the user saves */
    onSave: (value: string) => Promise<void>
    /** Icon to show before the display value */
    icon?: ReactNode
    /** Alignment of the popover */
    align?: "start" | "center" | "end"
    /** Additional class name for the trigger button */
    triggerClassName?: string
    /** Custom children to render as the trigger label */
    children?: ReactNode
}

/**
 * Converts an ISO datetime string to the format required by <input type="datetime-local">.
 * datetime-local expects "YYYY-MM-DDTHH:mm" (no seconds, no timezone).
 */
function toDateTimeLocalValue(iso: string): string {
    const date = new Date(iso)
    if (Number.isNaN(date.getTime())) return ""
    // Format as local datetime for the input
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, "0")
    const day = String(date.getDate()).padStart(2, "0")
    const hours = String(date.getHours()).padStart(2, "0")
    const minutes = String(date.getMinutes()).padStart(2, "0")
    return `${year}-${month}-${day}T${hours}:${minutes}`
}

/**
 * Converts a datetime-local value ("YYYY-MM-DDTHH:mm") back to an ISO string.
 */
function fromDateTimeLocalValue(value: string): string {
    const date = new Date(value)
    return date.toISOString()
}

export function DateTimeEdit({
    value,
    onSave,
    icon,
    align = "start",
    triggerClassName,
    children,
}: DateTimeEditProps) {
    const { t } = useTranslation()
    const [isOpen, setIsOpen] = useState(false)
    const [editValue, setEditValue] = useState(() => toDateTimeLocalValue(value))
    const [isSaving, setIsSaving] = useState(false)

    const handleSave = async () => {
        if (!editValue) return
        const newIso = fromDateTimeLocalValue(editValue)
        if (newIso === new Date(value).toISOString()) {
            setIsOpen(false)
            return
        }
        setIsSaving(true)
        try {
            await onSave(newIso)
            setIsOpen(false)
        } finally {
            setIsSaving(false)
        }
    }

    return (
        <Popover
            open={isOpen}
            onOpenChange={(open) => {
                setIsOpen(open)
                if (open) setEditValue(toDateTimeLocalValue(value))
            }}
        >
            <PopoverTrigger asChild>
                <button
                    type="button"
                    className={cn(
                        "inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group",
                        triggerClassName,
                    )}
                >
                    {children ?? (
                        <>
                            {icon}
                            <span>{value}</span>
                        </>
                    )}
                    <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                </button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-3" align={align}>
                <form
                    onSubmit={(e) => {
                        e.preventDefault()
                        handleSave()
                    }}
                    className="flex flex-col gap-2"
                >
                    <Input
                        type="datetime-local"
                        value={editValue}
                        onChange={(e) => setEditValue(e.target.value)}
                        autoFocus
                        required
                        disabled={isSaving}
                    />
                    <div className="flex justify-end gap-2">
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => setIsOpen(false)}
                            disabled={isSaving}
                        >
                            {t("cancel")}
                        </Button>
                        <Button type="submit" size="sm" disabled={isSaving}>
                            {isSaving ? t("saving", "Saving...") : t("save")}
                        </Button>
                    </div>
                </form>
            </PopoverContent>
        </Popover>
    )
}
