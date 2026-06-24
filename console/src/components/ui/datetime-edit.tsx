import { type ReactNode, useState } from "react"
import { Pencil } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/utils"
import {
    DEFAULT_TIME_INPUT_VALUE,
    dateInputValueFromIso,
    timeInputValueFromIso,
    toIsoFromDateAndTime,
} from "@/lib/date-time"
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
    const [editDate, setEditDate] = useState(() => dateInputValueFromIso(value))
    const [editTime, setEditTime] = useState(() => timeInputValueFromIso(value))
    const [isSaving, setIsSaving] = useState(false)

    const handleSave = async () => {
        const newIso = toIsoFromDateAndTime(editDate, editTime)
        if (!newIso) return

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
                if (open) {
                    setEditDate(dateInputValueFromIso(value))
                    setEditTime(timeInputValueFromIso(value))
                }
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
                    <div className="flex gap-2">
                        <Input
                            type="date"
                            value={editDate}
                            onChange={(e) => {
                                const nextDate = e.target.value
                                setEditDate(nextDate)
                                if (!nextDate) setEditTime(DEFAULT_TIME_INPUT_VALUE)
                            }}
                            autoFocus
                            required
                            disabled={isSaving}
                        />
                        <Input
                            type="time"
                            value={editTime}
                            onChange={(e) => setEditTime(e.target.value)}
                            disabled={isSaving || !editDate}
                        />
                    </div>
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
