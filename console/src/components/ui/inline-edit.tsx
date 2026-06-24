import { type ReactNode, useState } from "react"
import { Pencil } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import type { ZodSchema } from "zod/v4"

interface InlineEditProps {
    /** Current value to display and edit */
    value: string
    /** Called with the new value when the user saves. Return a promise to show a saving state. */
    onSave: (value: string) => Promise<void>
    /** Icon to show before the display value */
    icon?: ReactNode
    /** Placeholder text shown when value is empty */
    placeholder?: string
    /** Input type (e.g. "text", "email", "tel") */
    type?: string
    /** Input placeholder shown inside the popover */
    inputPlaceholder?: string
    /** Whether the input is required */
    required?: boolean
    /** Zod schema to validate the value before saving */
    validate?: ZodSchema
    /** Alignment of the popover */
    align?: "start" | "center" | "end"
    /** Additional class name for the trigger button */
    triggerClassName?: string
    /** Additional class name for the display text */
    textClassName?: string
    /** Size of the pencil icon */
    pencilSize?: string
    /** Custom children to render as the trigger label instead of the default icon + value */
    children?: ReactNode
    /** Accessible label for the trigger button */
    "aria-label"?: string
}

export function InlineEdit({
    value,
    onSave,
    icon,
    placeholder,
    type = "text",
    inputPlaceholder,
    required,
    validate,
    align = "start",
    triggerClassName,
    textClassName,
    pencilSize = "h-2.5 w-2.5",
    children,
    "aria-label": ariaLabel,
}: InlineEditProps) {
    const { t } = useTranslation()
    const [isOpen, setIsOpen] = useState(false)
    const [editValue, setEditValue] = useState(value)
    const [isSaving, setIsSaving] = useState(false)
    const [validationError, setValidationError] = useState<string | null>(null)

    const handleSave = async () => {
        const trimmed = editValue.trim()
        if (trimmed === value) {
            setIsOpen(false)
            return
        }

        if (validate) {
            const result = validate.safeParse(trimmed)
            if (!result.success) {
                setValidationError(result.error.issues[0]?.message || "Invalid value")
                return
            }
        }

        setValidationError(null)
        setIsSaving(true)
        try {
            await onSave(trimmed)
            setIsOpen(false)
        } finally {
            setIsSaving(false)
        }
    }

    const isEmpty = !value

    return (
        <Popover
            open={isOpen}
            onOpenChange={(open) => {
                setIsOpen(open)
                if (open) setEditValue(value)
            }}
        >
            <PopoverTrigger asChild>
                <button
                    type="button"
                    aria-label={ariaLabel ?? t("edit_value", "Edit value")}
                    className={cn(
                        "inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group",
                        isEmpty && "text-muted-foreground/60 hover:text-muted-foreground",
                        triggerClassName,
                    )}
                >
                    {children ?? (
                        <>
                            {icon}
                            <span className={textClassName}>{value || placeholder}</span>
                        </>
                    )}
                    <Pencil
                        className={cn(
                            pencilSize,
                            "opacity-0 group-hover:opacity-60 transition-opacity",
                        )}
                    />
                </button>
            </PopoverTrigger>
            <PopoverContent className="w-72 p-3" align={align}>
                <form
                    onSubmit={(e) => {
                        e.preventDefault()
                        handleSave()
                    }}
                    className="flex flex-col gap-2"
                >
                    <Input
                        type={type}
                        value={editValue}
                        onChange={(e) => {
                            setEditValue(e.target.value)
                            if (validationError) setValidationError(null)
                        }}
                        placeholder={inputPlaceholder}
                        autoFocus
                        required={required}
                        disabled={isSaving}
                        aria-invalid={!!validationError}
                    />
                    {validationError && (
                        <p className="text-sm text-destructive">{validationError}</p>
                    )}
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
