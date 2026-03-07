import * as React from "react"
import { Check, ChevronsUpDown, Loader2 } from "lucide-react"
import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Input } from "./input"

interface PathOption {
    path: string
}

interface ComboboxBaseProps<T> {
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

interface StaticComboboxProps<T extends PathOption> extends ComboboxBaseProps<T> {
    options: T[]
    onSearch?: never
    displayValue?: never
}

interface AsyncComboboxProps<T extends PathOption> extends ComboboxBaseProps<T> {
    options?: never
    onSearch: (query: string) => Promise<T[]>
    displayValue?: string
}

type ComboboxProps<T extends PathOption> = StaticComboboxProps<T> | AsyncComboboxProps<T>

export function Combobox<T extends PathOption>({
    options,
    onSearch,
    displayValue,
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
    const inputRef = React.useRef<HTMLInputElement>(null)
    const isAsync = !!onSearch

    // Static mode state
    const [inputValue, setInputValue] = React.useState(value || "")

    // Async mode state
    const [searchQuery, setSearchQuery] = React.useState("")
    const [searchResults, setSearchResults] = React.useState<T[]>([])
    const [loading, setLoading] = React.useState(false)
    const debounceRef = React.useRef<ReturnType<typeof setTimeout>>()

    // Sync input value with prop value (static mode)
    React.useEffect(() => {
        if (!isAsync) {
            setInputValue(value || "")
        }
    }, [value, isAsync])

    // Async search with debounce
    React.useEffect(() => {
        if (!isAsync || !open) return

        if (debounceRef.current) clearTimeout(debounceRef.current)

        setLoading(true)
        debounceRef.current = setTimeout(async () => {
            try {
                const results = await onSearch(searchQuery)
                setSearchResults(results)
            } finally {
                setLoading(false)
            }
        }, 300)

        return () => {
            if (debounceRef.current) clearTimeout(debounceRef.current)
        }
    }, [searchQuery, isAsync, open, onSearch])

    // Load initial results when popover opens in async mode
    React.useEffect(() => {
        if (isAsync && open && searchResults.length === 0) {
            setSearchQuery("")
        }
    }, [open, isAsync]) // eslint-disable-line react-hooks/exhaustive-deps

    // Static mode handlers
    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setInputValue(e.target.value)
    }

    const handleInputBlur = () => {
        if (inputValue !== value) {
            onValueChange(inputValue)
        }
    }

    const handleSelect = (selectedValue: string) => {
        if (isAsync) {
            onValueChange(selectedValue)
            setSearchQuery("")
        } else {
            setInputValue(selectedValue)
            onValueChange(selectedValue)
        }
        setOpen(false)
    }

    // Static mode: client-side filtering
    const filteredOptions = React.useMemo(() => {
        if (isAsync) return []
        if (!options) return []
        if (!inputValue) return options
        const searchLower = inputValue.toLowerCase()
        return options.filter((option) => option.path.toLowerCase().includes(searchLower))
    }, [options, inputValue, isAsync])

    if (isAsync) {
        const triggerLabel = displayValue || placeholder

        return (
            <div className={cn("relative flex", className)}>
                <Popover open={open} onOpenChange={setOpen}>
                    <PopoverTrigger asChild>
                        <Button
                            variant="outline"
                            role="combobox"
                            aria-expanded={open}
                            type="button"
                            disabled={disabled}
                            className={cn(
                                "h-8 w-full justify-between shadow-none font-normal",
                                !displayValue && "text-muted-foreground",
                                buttonClassName,
                            )}
                        >
                            <span className="truncate">{triggerLabel}</span>
                            <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                        </Button>
                    </PopoverTrigger>
                    <PopoverContent
                        className={cn("p-0", contentClassName)}
                        align="start"
                        onOpenAutoFocus={(e) => e.preventDefault()}
                    >
                        <Command shouldFilter={false}>
                            <CommandInput
                                placeholder={placeholder}
                                value={searchQuery}
                                onValueChange={setSearchQuery}
                            />
                            <CommandList>
                                {loading ? (
                                    <div className="flex items-center justify-center py-6">
                                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                                    </div>
                                ) : (
                                    <>
                                        <CommandEmpty>{emptyText}</CommandEmpty>
                                        <CommandGroup>
                                            {searchResults.map((option) => (
                                                <CommandItem
                                                    key={option.path}
                                                    value={option.path}
                                                    onSelect={handleSelect}
                                                    className="cursor-pointer"
                                                >
                                                    <Check
                                                        className={cn(
                                                            "mr-2 h-4 w-4",
                                                            value === option.path
                                                                ? "opacity-100"
                                                                : "opacity-0",
                                                        )}
                                                    />
                                                    {renderOption
                                                        ? renderOption(option, searchQuery)
                                                        : option.path}
                                                </CommandItem>
                                            ))}
                                        </CommandGroup>
                                    </>
                                )}
                            </CommandList>
                        </Command>
                    </PopoverContent>
                </Popover>
            </div>
        )
    }

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
