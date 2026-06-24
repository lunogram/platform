import * as React from "react"
import { HexColorPicker } from "react-colorful"
import { Popover, PopoverContent, PopoverTrigger } from "./popover"
import { cn } from "@/utils"

interface ThemeColor {
    label: string
    value: string
}

interface ColorPickerProps {
    value?: string
    onChange: (color: string) => void
    onClear?: () => void
    themeColors?: ThemeColor[]
    className?: string
    /** Render as inline row (swatch + hex input) matching PropertyPanel style */
    inline?: boolean
}

const DEFAULT_THEME_COLORS: ThemeColor[] = [
    { label: "Black", value: "#000000" },
    { label: "White", value: "#ffffff" },
    { label: "Gray", value: "#6b7280" },
    { label: "Red", value: "#ef4444" },
    { label: "Orange", value: "#f97316" },
    { label: "Amber", value: "#f59e0b" },
    { label: "Yellow", value: "#eab308" },
    { label: "Lime", value: "#84cc16" },
    { label: "Green", value: "#22c55e" },
    { label: "Emerald", value: "#10b981" },
    { label: "Teal", value: "#14b8a6" },
    { label: "Cyan", value: "#06b6d4" },
    { label: "Sky", value: "#0ea5e9" },
    { label: "Blue", value: "#3b82f6" },
    { label: "Indigo", value: "#6366f1" },
    { label: "Violet", value: "#8b5cf6" },
    { label: "Purple", value: "#a855f7" },
    { label: "Fuchsia", value: "#d946ef" },
    { label: "Pink", value: "#ec4899" },
    { label: "Rose", value: "#f43f5e" },
]

export function ColorPicker({
    value = "#000000",
    onChange,
    onClear,
    themeColors = DEFAULT_THEME_COLORS,
    className,
    inline = false,
}: ColorPickerProps) {
    const [localHex, setLocalHex] = React.useState(() => (value || "#000000").replace("#", ""))
    const [isOpen, setIsOpen] = React.useState(false)

    React.useEffect(() => {
        setLocalHex((value || "#000000").replace("#", ""))
    }, [value])

    const handlePickerChange = (color: string) => {
        setLocalHex(color.replace("#", ""))
        onChange(color)
    }

    const handleHexInput = (e: React.ChangeEvent<HTMLInputElement>) => {
        const raw = e.target.value.replace("#", "").slice(0, 6)
        setLocalHex(raw)
        if (/^[0-9A-F]{6}$/i.test(raw)) {
            onChange(`#${raw}`)
        }
    }

    const handleHexBlur = () => {
        if (!/^[0-9A-F]{6}$/i.test(localHex)) {
            setLocalHex((value || "#000000").replace("#", ""))
        }
    }

    const swatchGrid = themeColors.length > 0 && (
        <div className="grid grid-cols-10 gap-1 pt-2">
            {themeColors.map((c) => (
                <button
                    key={c.value}
                    type="button"
                    title={c.label}
                    className={cn(
                        "h-5 w-5 rounded-sm border transition-transform hover:scale-110",
                        value?.toLowerCase() === c.value.toLowerCase()
                            ? "border-foreground ring-1 ring-foreground/30"
                            : "border-white/20",
                    )}
                    style={{ backgroundColor: c.value }}
                    onClick={() => {
                        handlePickerChange(c.value)
                    }}
                />
            ))}
        </div>
    )

    // Inline variant: matches the PropertyPanel InlineColorInput layout
    if (inline) {
        return (
            <Popover open={isOpen} onOpenChange={setIsOpen}>
                <PopoverTrigger asChild>
                    <div
                        className={cn(
                            "flex items-center h-8 rounded-md bg-muted/50 overflow-hidden cursor-pointer",
                            className,
                        )}
                    >
                        <span className="flex items-center justify-center w-8 h-full shrink-0">
                            <span
                                className="h-4 w-4 rounded-sm border border-white/20"
                                style={{ backgroundColor: value || "#000000" }}
                            />
                        </span>
                        <span className="flex-1 h-full flex items-center text-sm px-1 font-mono truncate">
                            {localHex}
                        </span>
                        {onClear && (
                            <button
                                className="flex items-center justify-center w-7 h-full text-muted-foreground hover:text-foreground transition-colors shrink-0"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onClear()
                                }}
                                title="Remove color"
                            >
                                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                                    <line
                                        x1="2"
                                        y1="6"
                                        x2="10"
                                        y2="6"
                                        stroke="currentColor"
                                        strokeWidth="1.5"
                                        strokeLinecap="round"
                                    />
                                </svg>
                            </button>
                        )}
                    </div>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-3 space-y-2" align="end" side="left">
                    <HexColorPicker color={value || "#000000"} onChange={handlePickerChange} />
                    <div className="flex items-center gap-1.5">
                        <span className="text-xs text-muted-foreground">#</span>
                        <input
                            type="text"
                            value={localHex}
                            onChange={handleHexInput}
                            onBlur={handleHexBlur}
                            className="flex-1 h-7 bg-muted/50 rounded-md text-sm px-2 font-mono outline-none"
                            maxLength={6}
                        />
                    </div>
                    {swatchGrid}
                </PopoverContent>
            </Popover>
        )
    }

    // Standard variant (standalone)
    return (
        <div className={cn("flex gap-2", className)}>
            <Popover open={isOpen} onOpenChange={setIsOpen}>
                <PopoverTrigger asChild>
                    <button
                        type="button"
                        className="h-8 w-12 rounded border border-gray-300 shadow-sm cursor-pointer hover:border-gray-400 transition-colors flex-shrink-0"
                        style={{ backgroundColor: value }}
                        aria-label="Pick a color"
                    />
                </PopoverTrigger>
                <PopoverContent className="w-auto p-3 space-y-2" align="start">
                    <HexColorPicker color={value} onChange={handlePickerChange} />
                    <div className="flex items-center gap-1.5">
                        <span className="text-xs text-muted-foreground">#</span>
                        <input
                            type="text"
                            value={localHex}
                            onChange={handleHexInput}
                            onBlur={handleHexBlur}
                            className="flex-1 h-7 bg-muted/50 rounded-md text-sm px-2 font-mono outline-none"
                            maxLength={6}
                        />
                    </div>
                    {swatchGrid}
                </PopoverContent>
            </Popover>
        </div>
    )
}

export { DEFAULT_THEME_COLORS }
export type { ThemeColor, ColorPickerProps }
