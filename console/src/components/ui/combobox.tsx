import * as React from "react"
import { ChevronsUpDown } from "lucide-react"
import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Input } from "./input"

interface PathOption {
    path: string
}

interface ComboboxProps<T extends PathOption> {
    options: T[]
    value?: string
    onValueChange: (value: string) => void
    placeholder?: string
    emptyText?: string
    className?: string
    inputClassName?: string
    buttonClassName?: string
    contentClassName?: string
    disabled?: boolean
    required?: boolean
    renderOption?: (option: T, search: string) => React.ReactNode
}

export function Combobox<T extends PathOption>({
    options,
    value,
    onValueChange,
    placeholder = "Select option...",
    emptyText = "No results found.",
    className,
    inputClassName,
    buttonClassName,
    contentClassName,
    disabled = false,
    required = false,
    renderOption,
}: ComboboxProps<T>) {
    const [open, setOpen] = React.useState(false)
    const [inputValue, setInputValue] = React.useState(value || "")
    const inputRef = React.useRef<HTMLInputElement>(null)

    // Sync input value with prop value
    React.useEffect(() => {
        setInputValue(value || "")
    }, [value])

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const newValue = e.target.value
        setInputValue(newValue)
        // Don't call onValueChange while typing, only when selecting from dropdown
        // or when user finishes typing (blur)
    }

    const handleInputBlur = () => {
        // Only update if the input value matches exactly or is a custom value
        if (inputValue !== value) {
            onValueChange(inputValue)
        }
    }

    const handleSelect = (selectedValue: string) => {
        setInputValue(selectedValue)
        onValueChange(selectedValue)
        setOpen(false)
    }

    const filteredOptions = React.useMemo(() => {
        if (!inputValue) return options
        const searchLower = inputValue.toLowerCase()
        return options.filter((option) => option.path.toLowerCase().includes(searchLower))
    }, [options, inputValue])

    return (
        <div className={cn("relative flex", className)}>
            <Popover open={open} onOpenChange={setOpen}>
                <PopoverTrigger asChild>
                    <div className="flex">
                        <Input
                            ref={inputRef}
                            type="text"
                            value={inputValue}
                            onChange={handleInputChange}
                            onBlur={handleInputBlur}
                            required={required}
                            disabled={disabled}
                            placeholder={placeholder}
                            className={cn(
                                "h-8 rounded-l-md rounded-r-none shadow-none",
                                inputClassName,
                            )}
                        />
                        <Button
                            variant="outline"
                            role="combobox"
                            aria-expanded={open}
                            type="button"
                            disabled={disabled}
                            className={cn(
                                "h-8 w-9 rounded-r-md rounded-l-none border-l-0 px-0 shadow-none",
                                buttonClassName,
                            )}
                        >
                            <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
                        </Button>
                    </div>
                </PopoverTrigger>
                <PopoverContent
                    className={cn("p-0", contentClassName)}
                    align="start"
                    onOpenAutoFocus={(e) => e.preventDefault()}
                >
                    <Command>
                        <CommandList>
                            <CommandEmpty>{emptyText}</CommandEmpty>
                            <CommandGroup>
                                {filteredOptions.map((option) => (
                                    <CommandItem
                                        key={option.path}
                                        value={option.path}
                                        onSelect={handleSelect}
                                        className="cursor-pointer"
                                    >
                                        {renderOption
                                            ? renderOption(option, inputValue)
                                            : option.path}
                                    </CommandItem>
                                ))}
                            </CommandGroup>
                        </CommandList>
                    </Command>
                </PopoverContent>
            </Popover>
        </div>
    )
}
