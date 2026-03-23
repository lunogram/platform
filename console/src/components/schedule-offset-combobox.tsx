import * as React from "react"
import { Check, ChevronsUpDown, Loader2, Plus, ArrowLeft, Clock } from "lucide-react"
import { useTranslation } from "react-i18next"
import { cn, formatOffset } from "@/utils"
import { Button } from "@/components/ui/button"
import { Command, CommandGroup, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { oapiClient } from "@/oapi/client"
import type { ScheduleOffset } from "@/types"
import type { UUID } from "@/types/common"

type Direction = "before" | "after"
type TimeUnit = "minutes" | "hours" | "days" | "months"

/** Map UI unit names to PG INTERVAL unit strings. */
const UNIT_TO_PG: Record<TimeUnit, string> = {
    minutes: "minutes",
    hours: "hours",
    days: "days",
    months: "months",
}

/**
 * Build a PG INTERVAL offset string from the create-form inputs.
 * e.g. amount=30, unit="minutes" → "30 minutes"
 *      amount=1,  unit="hours"   → "1 hour"
 *      amount=0 (any)            → "0 minutes"
 */
function buildOffsetString(amount: number, unit: TimeUnit): string {
    if (amount === 0) return "0 minutes"
    const pgUnit = UNIT_TO_PG[unit]
    // Singularize: "minutes" → "minute" when amount is 1
    const label = amount === 1 ? pgUnit.replace(/s$/, "") : pgUnit
    return `${Math.round(amount)} ${label}`
}

interface ScheduleOffsetComboboxProps {
    projectId: string
    scheduledId: UUID
    offsets: ScheduleOffset[]
    value?: UUID
    onChange: (offsetId: UUID, offset: string, direction: string) => void
    onOffsetsChange: (offsets: ScheduleOffset[]) => void
    placeholder?: string
    disabled?: boolean
    className?: string
}

type PopoverView = "list" | "create"

export function ScheduleOffsetCombobox({
    projectId,
    scheduledId,
    offsets,
    value,
    onChange,
    onOffsetsChange,
    placeholder,
    disabled = false,
    className,
}: ScheduleOffsetComboboxProps) {
    const { t } = useTranslation()
    const [open, setOpen] = React.useState(false)

    // Inline creation state
    const [view, setView] = React.useState<PopoverView>("list")
    const [direction, setDirection] = React.useState<Direction>("before")
    const [amount, setAmount] = React.useState("")
    const [unit, setUnit] = React.useState<TimeUnit>("minutes")
    const [creating, setCreating] = React.useState(false)
    const [createError, setCreateError] = React.useState<string | null>(null)

    const selectedOffset = React.useMemo(
        () => offsets.find((o) => o.id === value) ?? null,
        [offsets, value],
    )

    const displayValue = React.useMemo(() => {
        if (!selectedOffset) return null
        return formatOffset(selectedOffset.offset, selectedOffset.direction)
    }, [selectedOffset])

    // Computed offset string for preview
    const computedOffset = React.useMemo(() => {
        const num = parseFloat(amount)
        if (amount === "" || isNaN(num) || num < 0) return null
        return buildOffsetString(num, unit)
    }, [amount, unit])

    const canSave = amount !== "" && !isNaN(parseFloat(amount)) && parseFloat(amount) >= 0

    const resetCreateForm = React.useCallback(() => {
        setDirection("before")
        setAmount("")
        setUnit("minutes")
        setCreateError(null)
        setCreating(false)
    }, [])

    // Reset view when popover opens
    React.useEffect(() => {
        if (open) {
            setView("list")
            resetCreateForm()
        }
    }, [open, resetCreateForm])

    const handleSwitchToCreate = () => {
        resetCreateForm()
        setView("create")
    }

    const handleCancelCreate = () => {
        setView("list")
        resetCreateForm()
    }

    const handleSelect = (offsetId: UUID) => {
        const offset = offsets.find((o) => o.id === offsetId)
        if (offset) {
            onChange(offsetId, offset.offset, offset.direction)
        }
        setOpen(false)
    }

    const handleCreate = async () => {
        if (computedOffset == null) return

        setCreating(true)
        setCreateError(null)

        try {
            const { data, error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/subjects/user/scheduled/schema/{scheduledID}/offsets",
                {
                    params: { path: { projectID: projectId, scheduledID: scheduledId } },
                    body: { offset: computedOffset, direction },
                },
            )
            if (error) throw error
            const created: ScheduleOffset = {
                ...data!,
                direction,
            }
            const updated = [...offsets, created]
            onOffsetsChange(updated)
            onChange(created.id, created.offset, created.direction)
            setOpen(false)
        } catch {
            setCreateError(
                t("schedule_offset_create_error", "Failed to create offset. It may already exist."),
            )
        } finally {
            setCreating(false)
        }
    }

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    type="button"
                    disabled={disabled}
                    className={cn(
                        "h-9 w-full justify-between shadow-sm font-normal",
                        !displayValue && "text-muted-foreground",
                        className,
                    )}
                >
                    <span className="flex items-center gap-2 truncate">
                        <Clock className="h-4 w-4 shrink-0 text-muted-foreground" />
                        <span className="font-sans">
                            {displayValue ??
                                (placeholder || t("select_offset", "Select an offset..."))}
                        </span>
                    </span>
                    <ChevronsUpDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                </Button>
            </PopoverTrigger>
            <PopoverContent
                className="p-0 w-[var(--radix-popover-trigger-width)]"
                align="start"
                onOpenAutoFocus={(e) => e.preventDefault()}
            >
                {view === "list" ? (
                    <ListView
                        offsets={offsets}
                        value={value}
                        onSelect={handleSelect}
                        onAddNew={handleSwitchToCreate}
                        t={t}
                    />
                ) : (
                    <CreateView
                        direction={direction}
                        onDirectionChange={setDirection}
                        amount={amount}
                        onAmountChange={setAmount}
                        unit={unit}
                        onUnitChange={setUnit}
                        computedOffset={computedOffset}
                        canSave={canSave}
                        onSave={handleCreate}
                        onCancel={handleCancelCreate}
                        creating={creating}
                        error={createError}
                        t={t}
                    />
                )}
            </PopoverContent>
        </Popover>
    )
}

interface ListViewProps {
    offsets: ScheduleOffset[]
    value?: UUID
    onSelect: (id: UUID) => void
    onAddNew: () => void
    t: (key: string, fallback?: string) => string
}

function ListView({ offsets, value, onSelect, onAddNew, t }: ListViewProps) {
    return (
        <div>
            <Command shouldFilter={false}>
                <CommandList>
                    {offsets.length === 0 ? (
                        <div className="flex flex-col items-center gap-1 py-6">
                            <Clock className="h-5 w-5 text-muted-foreground/50" />
                            <p className="text-sm text-muted-foreground">
                                {t("no_offsets_configured", "No offsets configured yet.")}
                            </p>
                            <p className="text-xs text-muted-foreground">
                                {t("add_offset_to_start", "Add one to get started.")}
                            </p>
                        </div>
                    ) : (
                        <CommandGroup className="max-h-64 overflow-y-auto">
                            {offsets.map((offset) => (
                                <CommandItem
                                    key={offset.id}
                                    value={offset.id}
                                    onSelect={() => onSelect(offset.id)}
                                    className="cursor-pointer"
                                >
                                    <div className="flex items-center gap-2 w-full">
                                        <Check
                                            className={cn(
                                                "h-4 w-4 shrink-0",
                                                value === offset.id ? "opacity-100" : "opacity-0",
                                            )}
                                        />
                                        <span className="truncate flex-1 text-sm">
                                            {formatOffset(offset.offset, offset.direction)}
                                        </span>
                                    </div>
                                </CommandItem>
                            ))}
                        </CommandGroup>
                    )}
                </CommandList>
            </Command>
            <div className="border-t p-1">
                <button
                    type="button"
                    onClick={onAddNew}
                    className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-pointer"
                >
                    <Plus className="h-4 w-4" />
                    {t("create_new_offset", "Create new offset...")}
                </button>
            </div>
        </div>
    )
}

interface CreateViewProps {
    direction: Direction
    onDirectionChange: (d: Direction) => void
    amount: string
    onAmountChange: (v: string) => void
    unit: TimeUnit
    onUnitChange: (u: TimeUnit) => void
    computedOffset: string | null
    canSave: boolean
    onSave: () => void
    onCancel: () => void
    creating: boolean
    error: string | null
    t: (key: string, fallback?: string) => string
}

const DIRECTION_VALUES: Direction[] = ["before", "after"]
const UNIT_VALUES: TimeUnit[] = ["minutes", "hours", "days", "months"]

function getDirectionLabel(d: Direction, t: (key: string, fallback?: string) => string): string {
    const labels: Record<Direction, [string, string]> = {
        before: ["offset_direction_before", "Before"],
        after: ["offset_direction_after", "After"],
    }
    return t(labels[d][0], labels[d][1])
}

function getUnitLabel(u: TimeUnit, t: (key: string, fallback?: string) => string): string {
    const labels: Record<TimeUnit, [string, string]> = {
        minutes: ["offset_unit_minutes", "Minutes"],
        hours: ["offset_unit_hours", "Hours"],
        days: ["offset_unit_days", "Days"],
        months: ["offset_unit_months", "Months"],
    }
    return t(labels[u][0], labels[u][1])
}

function CreateView({
    direction,
    onDirectionChange,
    amount,
    onAmountChange,
    unit,
    onUnitChange,
    computedOffset,
    canSave,
    onSave,
    onCancel,
    creating,
    error,
    t,
}: CreateViewProps) {
    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === "Enter") {
            e.preventDefault()
            if (canSave) onSave()
        }
        if (e.key === "Escape") {
            e.preventDefault()
            onCancel()
        }
    }

    return (
        <div className="p-3 space-y-3">
            <div className="flex items-center gap-2">
                <button
                    type="button"
                    onClick={onCancel}
                    aria-label={t("back", "Back")}
                    className="p-1 rounded-sm hover:bg-accent text-muted-foreground cursor-pointer"
                >
                    <ArrowLeft className="h-4 w-4" />
                </button>
                <span className="text-sm font-medium">
                    {t("create_offset_title", "Create new offset")}
                </span>
            </div>

            {/* Single row: [Amount] [Unit] [Before/After] */}
            <div className="flex items-center gap-1.5">
                <Input
                    type="number"
                    min="0"
                    step="any"
                    value={amount}
                    onChange={(e) => onAmountChange(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder="0"
                    autoFocus
                    className="h-8 w-[70px] shrink-0"
                />
                <Select value={unit} onValueChange={(v) => onUnitChange(v as TimeUnit)}>
                    <SelectTrigger className="h-8 flex-1 min-w-0">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {UNIT_VALUES.map((u) => (
                            <SelectItem key={u} value={u}>
                                {getUnitLabel(u, t)}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <Select value={direction} onValueChange={(v) => onDirectionChange(v as Direction)}>
                    <SelectTrigger className="h-8 w-[90px] shrink-0">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {DIRECTION_VALUES.map((d) => (
                            <SelectItem key={d} value={d}>
                                {getDirectionLabel(d, t)}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </div>

            {/* Preview */}
            {computedOffset != null && (
                <p className="text-xs text-muted-foreground">
                    {formatOffset(computedOffset, direction)}
                </p>
            )}

            {error && <p className="text-xs text-destructive">{error}</p>}

            <Button
                type="button"
                size="sm"
                onClick={onSave}
                disabled={!canSave || creating}
                className="h-8"
            >
                {creating && <Loader2 className="h-3 w-3 animate-spin mr-1.5" />}
                {t("create_offset", "Create offset")}
            </Button>
        </div>
    )
}
