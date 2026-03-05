import * as React from "react"
import { useCallback, useEffect, useRef, useState } from "react"
import { cn } from "@/utils"
import { Input } from "@/components/ui/input"

interface VariableAutocompleteInputProps
    extends Omit<React.ComponentProps<"input">, "onChange"> {
    variableNames: string[]
    value: string
    onChange: (value: string) => void
}

/**
 * An Input wrapper that shows a dropdown of variable names when the user types `{{`.
 * Selecting a variable inserts `{{ name }}` at the cursor position.
 */
const VariableAutocompleteInput = React.forwardRef<
    HTMLInputElement,
    VariableAutocompleteInputProps
>(({ variableNames, value, onChange, className, ...props }, ref) => {
    const [open, setOpen] = useState(false)
    const [filter, setFilter] = useState("")
    const [selectedIndex, setSelectedIndex] = useState(0)
    const [triggerPos, setTriggerPos] = useState<number | null>(null)
    const inputRef = useRef<HTMLInputElement | null>(null)
    const dropdownRef = useRef<HTMLDivElement>(null)

    // Merge forwarded ref with internal ref
    const setRefs = useCallback(
        (node: HTMLInputElement | null) => {
            inputRef.current = node
            if (typeof ref === "function") ref(node)
            else if (ref) (ref as React.MutableRefObject<HTMLInputElement | null>).current = node
        },
        [ref],
    )

    const filtered = React.useMemo(() => {
        if (!filter) return variableNames
        const lower = filter.toLowerCase()
        return variableNames.filter((n) => n.toLowerCase().includes(lower))
    }, [variableNames, filter])

    // Reset selected index when filtered list changes
    useEffect(() => {
        setSelectedIndex(0)
    }, [filtered.length])

    // Close dropdown when clicking outside
    useEffect(() => {
        if (!open) return
        const handler = (e: MouseEvent) => {
            if (
                dropdownRef.current &&
                !dropdownRef.current.contains(e.target as Node) &&
                inputRef.current &&
                !inputRef.current.contains(e.target as Node)
            ) {
                setOpen(false)
            }
        }
        document.addEventListener("mousedown", handler)
        return () => document.removeEventListener("mousedown", handler)
    }, [open])

    const insertVariable = useCallback(
        (name: string) => {
            if (triggerPos === null) return
            // Replace from the `{{` trigger position to current cursor
            const before = value.slice(0, triggerPos)
            const cursorPos = inputRef.current?.selectionStart ?? value.length
            const after = value.slice(cursorPos)
            const inserted = `{{ ${name} }}`
            const newValue = before + inserted + after
            onChange(newValue)
            setOpen(false)
            setFilter("")
            setTriggerPos(null)

            // Restore cursor position after the inserted text
            requestAnimationFrame(() => {
                const pos = before.length + inserted.length
                inputRef.current?.setSelectionRange(pos, pos)
                inputRef.current?.focus()
            })
        },
        [value, onChange, triggerPos],
    )

    const handleChange = useCallback(
        (e: React.ChangeEvent<HTMLInputElement>) => {
            const newValue = e.target.value
            const cursor = e.target.selectionStart ?? newValue.length
            onChange(newValue)

            if (variableNames.length === 0) return

            // Look backward from cursor for `{{`
            const textBefore = newValue.slice(0, cursor)
            const lastDoubleBrace = textBefore.lastIndexOf("{{")

            if (lastDoubleBrace !== -1) {
                // Check there's no `}}` between the `{{` and cursor
                const between = textBefore.slice(lastDoubleBrace + 2)
                if (!between.includes("}}")) {
                    // Extract the filter text (strip leading whitespace)
                    const filterText = between.trimStart()
                    setFilter(filterText)
                    setTriggerPos(lastDoubleBrace)
                    setOpen(true)
                    return
                }
            }

            setOpen(false)
            setFilter("")
            setTriggerPos(null)
        },
        [onChange, variableNames.length],
    )

    const handleKeyDown = useCallback(
        (e: React.KeyboardEvent<HTMLInputElement>) => {
            if (!open || filtered.length === 0) return

            switch (e.key) {
                case "ArrowDown":
                    e.preventDefault()
                    setSelectedIndex((prev) =>
                        prev < filtered.length - 1 ? prev + 1 : 0,
                    )
                    break
                case "ArrowUp":
                    e.preventDefault()
                    setSelectedIndex((prev) =>
                        prev > 0 ? prev - 1 : filtered.length - 1,
                    )
                    break
                case "Enter":
                    e.preventDefault()
                    insertVariable(filtered[selectedIndex])
                    break
                case "Escape":
                    e.preventDefault()
                    setOpen(false)
                    break
                case "Tab":
                    if (open) {
                        e.preventDefault()
                        insertVariable(filtered[selectedIndex])
                    }
                    break
            }
        },
        [open, filtered, selectedIndex, insertVariable],
    )

    // Don't render dropdown at all if no variables are defined
    if (variableNames.length === 0) {
        return (
            <Input
                ref={setRefs}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                className={className}
                {...props}
            />
        )
    }

    return (
        <div className={cn("relative", className)}>
            <Input
                ref={setRefs}
                value={value}
                onChange={handleChange}
                onKeyDown={handleKeyDown}
                role="combobox"
                aria-expanded={open && filtered.length > 0}
                aria-haspopup="listbox"
                aria-autocomplete="list"
                aria-controls={open ? "variable-listbox" : undefined}
                aria-activedescendant={open && filtered.length > 0 ? `variable-option-${selectedIndex}` : undefined}
                {...props}
            />
            {open && filtered.length > 0 && (
                <div
                    ref={dropdownRef}
                    id="variable-listbox"
                    role="listbox"
                    className="absolute left-0 top-full z-50 mt-1 w-full min-w-48 max-h-48 overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md animate-in fade-in-0 zoom-in-95"
                >
                    {filtered.map((name, i) => (
                        <button
                            key={name}
                            id={`variable-option-${i}`}
                            type="button"
                            role="option"
                            aria-selected={i === selectedIndex}
                            className={cn(
                                "flex w-full items-center rounded-sm px-2 py-1.5 text-xs cursor-pointer transition-colors",
                                i === selectedIndex
                                    ? "bg-accent text-accent-foreground"
                                    : "hover:bg-accent/50",
                            )}
                            onMouseDown={(e) => {
                                e.preventDefault()
                                insertVariable(name)
                            }}
                            onMouseEnter={() => setSelectedIndex(i)}
                        >
                            <span className="font-mono">{"{{ "}{name}{" }}"}</span>
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
})

VariableAutocompleteInput.displayName = "VariableAutocompleteInput"

export { VariableAutocompleteInput }
