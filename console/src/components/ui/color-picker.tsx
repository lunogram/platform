import * as React from "react"
import { HexColorPicker } from "react-colorful"
import { Popover, PopoverContent, PopoverTrigger } from "./popover"
import { Input } from "./input"
import { cn } from "@/utils"

interface ColorPickerProps {
    value?: string
    onChange: (color: string) => void
    className?: string
}

export function ColorPicker({ value = "#000000", onChange, className }: ColorPickerProps) {
    const [localValue, setLocalValue] = React.useState(value)
    const [isOpen, setIsOpen] = React.useState(false)

    React.useEffect(() => {
        setLocalValue(value)
    }, [value])

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const newValue = e.target.value
        setLocalValue(newValue)

        // Only call onChange if it's a valid hex color
        if (/^#[0-9A-F]{6}$/i.test(newValue)) {
            onChange(newValue)
        }
    }

    const handlePickerChange = (color: string) => {
        setLocalValue(color)
        onChange(color)
    }

    const handleInputBlur = () => {
        // Ensure we have a valid color on blur
        if (!/^#[0-9A-F]{6}$/i.test(localValue)) {
            setLocalValue(value)
        }
    }

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
                <PopoverContent className="w-auto p-3" align="start">
                    <HexColorPicker color={value} onChange={handlePickerChange} />
                </PopoverContent>
            </Popover>
            <Input
                type="text"
                value={localValue}
                onChange={handleInputChange}
                onBlur={handleInputBlur}
                placeholder="#000000"
                className="h-8 font-mono text-sm flex-1"
                maxLength={7}
            />
        </div>
    )
}
