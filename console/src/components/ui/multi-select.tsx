import * as React from "react"
import { X, Check, ChevronsUpDown } from "lucide-react"
import { cn } from "@/utils"
import { Badge } from "@/components/ui/badge"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

export interface MultiSelectOption<T = string> {
    value: T
    label: string
}

type MultiSelectElevation = "default" | "flat"

interface MultiSelectProps<T = string> {
    options: MultiSelectOption<T>[]
    value?: T[]
    onChange?: (value: T[]) => void
    placeholder?: string
    disabled?: boolean
    className?: string
    maxDisplay?: number
    elevation?: MultiSelectElevation
}

export function MultiSelect<T = string>({
    options,
    value = [],
    onChange,
    placeholder = "Select items...",
    disabled = false,
    className,
    maxDisplay = 3,
    elevation = "default",
}: MultiSelectProps<T>) {
    const [open, setOpen] = React.useState(false)

    const handleUnselect = (item: T) => {
        onChange?.(value.filter((i) => i !== item))
    }

    const handleSelect = (item: T) => {
        const isSelected = value.includes(item)
        if (isSelected) {
            onChange?.(value.filter((i) => i !== item))
        } else {
            onChange?.([...value, item])
        }
    }

    const selectedOptions = options.filter((option) => value.includes(option.value))

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <button
                    type="button"
                    role="combobox"
                    aria-expanded={open}
                    disabled={disabled}
                    className={cn(
                        "cursor-pointer flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-input bg-transparent px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
                        elevation === "default" ? "shadow-sm" : "shadow-none",
                        "select-button",
                        open && "border-blue-500 ring-1 ring-blue-500",
                        className,
                    )}
                >
                    <div className="flex gap-1 flex-wrap items-center flex-1 min-w-0">
                        {selectedOptions.length > 0 ? (
                            <>
                                {selectedOptions.slice(0, maxDisplay).map((option) => (
                                    <Badge
                                        variant="secondary"
                                        key={String(option.value)}
                                        className="mr-1 text-xs"
                                    >
                                        {option.label}
                                        <button
                                            type="button"
                                            className="ml-1 ring-offset-background rounded-full outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
                                            onKeyDown={(e) => {
                                                if (e.key === "Enter") {
                                                    e.stopPropagation()
                                                    handleUnselect(option.value)
                                                }
                                            }}
                                            onMouseDown={(e) => {
                                                e.preventDefault()
                                                e.stopPropagation()
                                            }}
                                            onClick={(e) => {
                                                e.preventDefault()
                                                e.stopPropagation()
                                                handleUnselect(option.value)
                                            }}
                                        >
                                            <X className="h-3 w-3 text-muted-foreground hover:text-foreground" />
                                        </button>
                                    </Badge>
                                ))}
                                {selectedOptions.length > maxDisplay && (
                                    <Badge variant="secondary" className="mr-1 text-xs">
                                        +{selectedOptions.length - maxDisplay} more
                                    </Badge>
                                )}
                            </>
                        ) : (
                            <span className="text-muted-foreground select-button-label">
                                {placeholder}
                            </span>
                        )}
                    </div>
                    <ChevronsUpDown
                        className={cn(
                            "h-[16px] w-4 shrink-0 opacity-50 select-button-icon transition-colors",
                            open && "text-blue-500 opacity-100",
                        )}
                    />
                </button>
            </PopoverTrigger>
            <PopoverContent
                className="w-[--radix-popover-trigger-width] p-0 select-options"
                align="start"
                side="bottom"
                sideOffset={4}
            >
                <Command>
                    <CommandInput placeholder="Search..." className="h-9" />
                    <CommandList>
                        <CommandEmpty>No results found.</CommandEmpty>
                        <CommandGroup>
                            {options.map((option) => {
                                const isSelected = value.includes(option.value)
                                return (
                                    <CommandItem
                                        key={String(option.value)}
                                        onSelect={() => handleSelect(option.value)}
                                        className="cursor-pointer select-option"
                                    >
                                        <div
                                            className={cn(
                                                "mr-2 flex h-4 w-4 items-center justify-center rounded-sm border border-primary",
                                                isSelected
                                                    ? "bg-primary text-primary-foreground"
                                                    : "opacity-50 [&_svg]:invisible",
                                            )}
                                        >
                                            <Check className="h-4 w-4" />
                                        </div>
                                        <span>{option.label}</span>
                                    </CommandItem>
                                )
                            })}
                        </CommandGroup>
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}
