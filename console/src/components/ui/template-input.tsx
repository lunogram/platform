import * as React from "react"
import { useCallback, useEffect, useRef, useState } from "react"
import { Braces } from "lucide-react"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { cn } from "@/utils"
import type { Variable, VariableGroup } from "@/views/journey/JourneyVariableContext"

// ── Token parsing ───────────────────────────────────────────────────

interface TextToken {
    type: "text"
    value: string
}
interface VarToken {
    type: "variable"
    value: string // raw path, e.g. "user.email"
}
type Token = TextToken | VarToken

const TOKEN_RE = /\{\{\s*(.*?)\s*\}\}/g

function tokenize(input: string): Token[] {
    const tokens: Token[] = []
    let last = 0
    for (const m of input.matchAll(TOKEN_RE)) {
        if (m.index > last) tokens.push({ type: "text", value: input.slice(last, m.index) })
        tokens.push({ type: "variable", value: m[1] })
        last = m.index + m[0].length
    }
    if (last < input.length) tokens.push({ type: "text", value: input.slice(last) })
    return tokens
}

// ── Zero-width space helpers ────────────────────────────────────────
// We insert zero-width spaces (ZWS) between and around pills so the
// browser always has a text node where the caret can land.

const ZWS = "\u200B"

function createPill(path: string): HTMLSpanElement {
    const pill = document.createElement("span")
    pill.contentEditable = "false"
    pill.dataset.variable = path
    pill.className =
        "inline-flex items-center gap-0.5 rounded bg-primary/10 text-primary px-1 py-0.5 text-xs font-mono whitespace-nowrap select-all mx-0.5 align-baseline"
    pill.textContent = `{{ ${path} }}`
    return pill
}

// ── Variants ────────────────────────────────────────────────────────

const variantStyles = {
    default: {
        editor: "min-h-9 rounded-md px-3 py-1.5 text-base shadow-sm md:text-sm",
        style: { minHeight: "2.25rem", lineHeight: "1.5rem" } as React.CSSProperties,
    },
    compact: {
        editor: "h-8 rounded-md px-2 py-1 text-xs",
        style: { minHeight: "2rem", lineHeight: "1.25rem" } as React.CSSProperties,
    },
}

// ── Component ───────────────────────────────────────────────────────

export interface TemplateInputProps {
    value: string
    onChange: (value: string) => void
    placeholder?: string
    variables?: VariableGroup[]
    className?: string
    disabled?: boolean
    id?: string
    /** Visual variant. "compact" matches small inline inputs (e.g. rule editor). */
    variant?: keyof typeof variantStyles
}

export function TemplateInput({
    value,
    onChange,
    placeholder,
    variables = [],
    className,
    disabled,
    id,
    variant = "default",
}: TemplateInputProps) {
    const [pickerOpen, setPickerOpen] = useState(false)
    const editorRef = useRef<HTMLDivElement>(null)
    const isComposing = useRef(false)

    // Keep a ref to the latest value so we can compare against DOM state
    const valueRef = useRef(value)
    valueRef.current = value

    // ── Sync DOM ← value (only when value diverges from what the user typed)
    useEffect(() => {
        const el = editorRef.current
        if (!el) return
        const domPlain = domToPlain(el)
        if (domPlain !== value) {
            renderTokens(el, value)
        }
    }, [value])

    // ── DOM → plain string ──────────────────────────────────────────
    function domToPlain(el: HTMLDivElement): string {
        let out = ""
        for (const child of el.childNodes) {
            if (child.nodeType === Node.TEXT_NODE) {
                // Strip zero-width spaces — they are only for caret positioning
                out += (child.textContent ?? "").replaceAll(ZWS, "")
            } else if (child instanceof HTMLElement) {
                if (child.dataset.variable) {
                    out += `{{ ${child.dataset.variable} }}`
                } else {
                    out += (child.textContent ?? "").replaceAll(ZWS, "")
                }
            }
        }
        return out
    }

    // ── Render tokens into the contentEditable ──────────────────────
    function renderTokens(el: HTMLDivElement, raw: string) {
        const tokens = tokenize(raw)

        el.innerHTML = ""

        // Always start with a text node so the cursor can be placed before
        // the first pill
        if (tokens.length === 0 || tokens[0]?.type === "variable") {
            el.appendChild(document.createTextNode(ZWS))
        }

        for (let i = 0; i < tokens.length; i++) {
            const tok = tokens[i]
            if (tok.type === "text") {
                el.appendChild(document.createTextNode(tok.value))
            } else {
                el.appendChild(createPill(tok.value))
                // After every pill, ensure there is a text node the caret can
                // land in. If the next token is also a variable (or this is the
                // last token) insert a ZWS text node.
                const next = tokens[i + 1]
                if (!next || next.type === "variable") {
                    el.appendChild(document.createTextNode(ZWS))
                }
            }
        }
    }

    // ── Handle input ────────────────────────────────────────────────
    const handleInput = useCallback(() => {
        if (isComposing.current) return
        const el = editorRef.current
        if (!el) return
        const plain = domToPlain(el)
        if (plain !== valueRef.current) {
            onChange(plain)
        }
    }, [onChange])

    // ── Insert a variable at cursor ─────────────────────────────────
    const insertVariable = useCallback(
        (path: string) => {
            const el = editorRef.current
            const snippet = `{{ ${path} }}`

            if (!el) {
                onChange(value + snippet)
                setPickerOpen(false)
                return
            }

            const sel = window.getSelection()
            if (sel && sel.rangeCount > 0) {
                const range = sel.getRangeAt(0)
                if (el.contains(range.commonAncestorContainer)) {
                    range.deleteContents()
                    const pill = createPill(path)

                    // Insert a ZWS text node after the pill so the cursor has
                    // somewhere to go
                    const afterText = document.createTextNode(ZWS)
                    range.insertNode(afterText)
                    range.insertNode(pill)

                    // Also ensure there is a text node before the pill if needed
                    if (!pill.previousSibling || pill.previousSibling instanceof HTMLElement) {
                        el.insertBefore(document.createTextNode(ZWS), pill)
                    }

                    // Move cursor into the text node after the pill
                    range.setStart(afterText, afterText.length)
                    range.collapse(true)
                    sel.removeAllRanges()
                    sel.addRange(range)

                    // Sync
                    const plain = domToPlain(el)
                    onChange(plain)
                    setPickerOpen(false)
                    return
                }
            }

            // Fallback — no valid cursor in our editor, append
            onChange(value + snippet)
            setPickerOpen(false)
            requestAnimationFrame(() => el.focus())
        },
        [onChange, value],
    )

    // ── Keyboard: prevent Enter (single-line) ───────────────────────
    const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
        if (e.key === "Enter") {
            e.preventDefault()
        }
    }, [])

    const hasVariables = variables.some((g) => g.variables.length > 0)
    const vs = variantStyles[variant]

    return (
        <div className={cn("relative flex items-center", className)}>
            {/* Editable area */}
            <div
                ref={editorRef}
                id={id}
                role="textbox"
                contentEditable={!disabled}
                suppressContentEditableWarning
                onInput={handleInput}
                onKeyDown={handleKeyDown}
                onCompositionStart={() => {
                    isComposing.current = true
                }}
                onCompositionEnd={() => {
                    isComposing.current = false
                    handleInput()
                }}
                data-placeholder={placeholder}
                className={cn(
                    "flex w-full border border-input bg-transparent transition-colors",
                    vs.editor,
                    "placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
                    "disabled:cursor-not-allowed disabled:opacity-50",
                    "empty:before:content-[attr(data-placeholder)] empty:before:text-muted-foreground empty:before:pointer-events-none",
                    "overflow-x-auto overflow-y-hidden whitespace-nowrap",
                    "[&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]",
                    hasVariables && "pr-9",
                    disabled && "cursor-not-allowed opacity-50",
                    className,
                )}
                style={vs.style}
            />

            {/* Variable picker trigger */}
            {hasVariables && !disabled && (
                <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
                    <PopoverTrigger asChild>
                        <button
                            type="button"
                            className={cn(
                                "absolute right-1.5 top-1/2 -translate-y-1/2 flex items-center justify-center",
                                "h-6 w-6 rounded text-muted-foreground hover:text-foreground hover:bg-muted transition-colors",
                            )}
                            tabIndex={-1}
                            aria-label="Insert variable"
                        >
                            <Braces className="h-3.5 w-3.5" />
                        </button>
                    </PopoverTrigger>
                    <PopoverContent
                        className="w-72 p-0"
                        align="end"
                        side="bottom"
                        onOpenAutoFocus={(e) => e.preventDefault()}
                    >
                        <Command>
                            <CommandInput placeholder="Search variables..." />
                            <CommandList>
                                <CommandEmpty>No variables found.</CommandEmpty>
                                {variables.map((group) => (
                                    <CommandGroup key={group.label} heading={group.label}>
                                        {group.variables.map((v) => (
                                            <CommandItem
                                                key={v.path}
                                                value={v.path}
                                                onSelect={() => insertVariable(v.path)}
                                            >
                                                <span className="font-mono text-xs">{v.label}</span>
                                                {v.description && (
                                                    <span className="ml-auto text-xs text-muted-foreground truncate max-w-[100px]">
                                                        {v.description}
                                                    </span>
                                                )}
                                            </CommandItem>
                                        ))}
                                    </CommandGroup>
                                ))}
                            </CommandList>
                        </Command>
                    </PopoverContent>
                </Popover>
            )}
        </div>
    )
}
