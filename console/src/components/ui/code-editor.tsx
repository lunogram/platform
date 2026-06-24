import { useEffect, useRef, useState, useCallback, useMemo } from "react"
import { EditorState, Compartment } from "@codemirror/state"
import {
    EditorView,
    keymap,
    lineNumbers,
    highlightActiveLine,
    highlightActiveLineGutter,
} from "@codemirror/view"
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands"
import { json } from "@codemirror/lang-json"
import { foldGutter, indentOnInput, bracketMatching, foldKeymap } from "@codemirror/language"
import { linter, lintGutter } from "@codemirror/lint"
import type { Diagnostic } from "@codemirror/lint"
import { autocompletion, completionKeymap } from "@codemirror/autocomplete"
import { githubLight } from "@fsegurai/codemirror-theme-github-light"
import { githubDark } from "@fsegurai/codemirror-theme-github-dark"
import { Copy, Check, WandSparkles, AlertCircle, Braces } from "lucide-react"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"
import { cn } from "@/utils"

type EditorLanguage = "json" | "auto" | "none"

interface CodeEditorProps {
    value: string
    onChange: (value: string) => void
    /** Called when the JSON validation state changes (only applies when language is "json"). */
    onError?: (error: string | null) => void
    className?: string
    minHeight?: number
    maxHeight?: number
    readOnly?: boolean
    placeholder?: string
    /**
     * Language mode for the editor.
     * - `"json"` — enables JSON syntax, linting, and validation
     * - `"auto"` — auto-detects JSON from content, no linting (default)
     * - `"none"` — plain text, no language support
     */
    language?: EditorLanguage
    /**
     * When true (default), the JSON linter requires the root value to be an
     * object (`{}`). Set to false to also allow arrays and other root types.
     * Only applies when `language` is `"json"`.
     */
    requireObject?: boolean
    /**
     * Optional variable groups for a picker button in the toolbar.
     * When provided (with at least one variable), a `{x}` button appears
     * that inserts `{{ path }}` at the cursor position.
     */
    variables?: VariableGroup[]
}

// Minimal overrides for integration
const baseTheme = EditorView.theme({
    "&": {
        fontSize: "13px",
        height: "100%",
    },
    "&.cm-focused": {
        outline: "none",
        boxShadow: "none",
    },
    ".cm-scroller": {
        overflow: "auto",
    },
    "&.cm-editor .cm-line": {
        wordBreak: "break-all",
    },
})

function isJsonContent(value: string): boolean {
    const trimmed = value.trim()
    if (!trimmed) return false
    return (
        (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
        (trimmed.startsWith("[") && trimmed.endsWith("]"))
    )
}

function canFormatAsJson(value: string): boolean {
    if (!isJsonContent(value)) return false
    try {
        JSON.parse(value)
        return true
    } catch {
        return false
    }
}

// Custom JSON linter that validates syntax and optionally structure
function createJsonLinter(requireObject: boolean) {
    return linter((view) => {
        const diagnostics: Diagnostic[] = []
        const doc = view.state.doc.toString()

        if (!doc.trim()) return diagnostics

        try {
            const parsed = JSON.parse(doc)
            if (
                requireObject &&
                (typeof parsed !== "object" || parsed === null || Array.isArray(parsed))
            ) {
                diagnostics.push({
                    from: 0,
                    to: doc.length,
                    severity: "error",
                    message: "Root must be a JSON object",
                })
            }
        } catch (e) {
            const message = e instanceof Error ? e.message : "Invalid JSON"
            const posMatch = message.match(/position\s+(\d+)/i)
            const pos = posMatch ? parseInt(posMatch[1], 10) : 0

            diagnostics.push({
                from: Math.min(pos, doc.length),
                to: Math.min(pos + 1, doc.length),
                severity: "error",
                message: message,
            })
        }

        return diagnostics
    })
}

export function CodeEditor({
    value,
    onChange,
    onError,
    className,
    minHeight = 100,
    maxHeight = 400,
    readOnly = false,
    placeholder,
    language = "auto",
    requireObject = true,
    variables = [],
}: CodeEditorProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const viewRef = useRef<EditorView | null>(null)
    const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const [copied, setCopied] = useState(false)
    const [varPickerOpen, setVarPickerOpen] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [isDark, setIsDark] = useState(() => document.documentElement.classList.contains("dark"))

    const isJsonMode = language === "json"

    // Create compartments once per component instance
    const themeCompartment = useMemo(() => new Compartment(), [])
    const editableCompartment = useMemo(() => new Compartment(), [])
    const languageCompartment = useMemo(() => new Compartment(), [])

    // Store callbacks in refs to avoid recreating the editor
    const onChangeRef = useRef(onChange)
    const validateJsonRef = useRef<(doc: string) => void>(() => {})

    useEffect(() => {
        onChangeRef.current = onChange
    }, [onChange])

    // Track dark mode changes
    useEffect(() => {
        const observer = new MutationObserver((mutations) => {
            mutations.forEach((mutation) => {
                if (mutation.attributeName === "class") {
                    setIsDark(document.documentElement.classList.contains("dark"))
                }
            })
        })
        observer.observe(document.documentElement, { attributes: true })
        return () => observer.disconnect()
    }, [])

    // Validate JSON and report errors (only used in json mode)
    const validateJson = useCallback(
        (doc: string) => {
            if (!isJsonMode) return

            if (!doc.trim()) {
                setError(null)
                onError?.(null)
                return
            }

            try {
                const parsed = JSON.parse(doc)
                if (
                    requireObject &&
                    (typeof parsed !== "object" || parsed === null || Array.isArray(parsed))
                ) {
                    const err = "Root must be a JSON object"
                    setError(err)
                    onError?.(err)
                } else {
                    setError(null)
                    onError?.(null)
                }
            } catch (e) {
                const err = e instanceof Error ? e.message : "Invalid JSON"
                setError(err)
                onError?.(err)
            }
        },
        [isJsonMode, onError, requireObject],
    )

    useEffect(() => {
        validateJsonRef.current = validateJson
    }, [validateJson])

    // Determine the initial language extensions
    const getLanguageExtensions = useCallback(
        (content: string) => {
            if (language === "json") return [json(), createJsonLinter(requireObject), lintGutter()]
            if (language === "auto" && isJsonContent(content)) return [json()]
            return []
        },
        [language, requireObject],
    )

    // Initialize editor
    useEffect(() => {
        if (!containerRef.current) return

        const updateListener = EditorView.updateListener.of((update) => {
            if (update.docChanged) {
                const doc = update.state.doc.toString()
                onChangeRef.current(doc)
                validateJsonRef.current(doc)
            }
        })

        const extensions = [
            lineNumbers(),
            highlightActiveLineGutter(),
            highlightActiveLine(),
            history(),
            foldGutter(),
            indentOnInput(),
            bracketMatching(),
            autocompletion(),
            languageCompartment.of(getLanguageExtensions(value)),
            keymap.of([
                ...defaultKeymap,
                ...historyKeymap,
                ...foldKeymap,
                ...completionKeymap,
                ...(isJsonMode ? [indentWithTab] : []),
            ]),
            baseTheme,
            themeCompartment.of(isDark ? githubDark : githubLight),
            editableCompartment.of(EditorView.editable.of(!readOnly)),
            ...(readOnly ? [EditorView.lineWrapping] : []),
            updateListener,
            ...(placeholder
                ? [EditorView.contentAttributes.of({ "aria-placeholder": placeholder })]
                : []),
            EditorView.contentAttributes.of({
                "aria-label": isJsonMode ? "JSON Editor" : "Code Editor",
            }),
        ]

        const state = EditorState.create({
            doc: value,
            extensions,
        })

        const view = new EditorView({
            state,
            parent: containerRef.current,
        })

        viewRef.current = view

        if (isJsonMode) {
            validateJson(value)
        }

        return () => {
            view.destroy()
            viewRef.current = null
            if (copyTimerRef.current) clearTimeout(copyTimerRef.current)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [themeCompartment, editableCompartment, languageCompartment])

    // Update theme when dark mode changes
    useEffect(() => {
        const view = viewRef.current
        if (!view) return

        view.dispatch({
            effects: themeCompartment.reconfigure(isDark ? githubDark : githubLight),
        })
    }, [isDark, themeCompartment])

    // Update editable state
    useEffect(() => {
        const view = viewRef.current
        if (!view) return

        view.dispatch({
            effects: editableCompartment.reconfigure(EditorView.editable.of(!readOnly)),
        })
    }, [readOnly, editableCompartment])

    // Sync external value changes & auto-detect language
    useEffect(() => {
        const view = viewRef.current
        if (!view) return

        const currentValue = view.state.doc.toString()
        if (value !== currentValue) {
            view.dispatch({
                changes: {
                    from: 0,
                    to: currentValue.length,
                    insert: value,
                },
            })
        }

        // Update language mode based on content (only in auto mode)
        if (language === "auto") {
            view.dispatch({
                effects: languageCompartment.reconfigure(isJsonContent(value) ? [json()] : []),
            })
        }
    }, [value, language, languageCompartment])

    const handleCopy = async () => {
        await navigator.clipboard.writeText(value)
        setCopied(true)
        if (copyTimerRef.current) clearTimeout(copyTimerRef.current)
        copyTimerRef.current = setTimeout(() => setCopied(false), 2000)
    }

    const handleFormat = () => {
        try {
            const parsed = JSON.parse(value)
            const formatted = JSON.stringify(parsed, null, 2)
            // Only update via onChange — the sync effect will push the
            // formatted value into the CodeMirror editor.
            onChange(formatted)
        } catch {
            // Can't format invalid JSON
        }
    }

    const hasVariables = variables.some((g) => g.variables.length > 0)

    const insertVariable = useCallback(
        (path: string) => {
            const view = viewRef.current
            const snippet = `{{ ${path} }}`
            if (view) {
                const { from, to } = view.state.selection.main
                view.dispatch({
                    changes: { from, to, insert: snippet },
                    selection: { anchor: from + snippet.length },
                })
                view.focus()
            } else {
                onChange(value + snippet)
            }
            setVarPickerOpen(false)
        },
        [onChange, value],
    )

    // Calculate dynamic height based on line count
    const lineCount = value.split("\n").length
    const lineHeight = 20
    const padding = 24
    const calculatedHeight = Math.min(
        Math.max(lineCount * lineHeight + padding, minHeight),
        maxHeight,
    )

    const showFormat = isJsonMode ? !error : canFormatAsJson(value)

    return (
        <div className={cn("relative rounded-md border", className)}>
            {/* Toolbar */}
            <div className="absolute top-2 right-2 z-10 flex items-center gap-1">
                {hasVariables && !readOnly && (
                    <Popover open={varPickerOpen} onOpenChange={setVarPickerOpen}>
                        <PopoverTrigger asChild>
                            <button
                                type="button"
                                className="p-1.5 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
                                title="Insert variable"
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
                                                    <span className="font-mono text-xs">
                                                        {v.label}
                                                    </span>
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
                {showFormat && !readOnly && (
                    <button
                        type="button"
                        onClick={handleFormat}
                        className="p-1.5 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
                        title="Format JSON"
                    >
                        <WandSparkles className="h-3.5 w-3.5" />
                    </button>
                )}
                <button
                    type="button"
                    onClick={handleCopy}
                    className="p-1.5 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
                    title="Copy to clipboard"
                >
                    {copied ? (
                        <Check className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                        <Copy className="h-3.5 w-3.5" />
                    )}
                </button>
            </div>

            {/* Editor */}
            <div
                ref={containerRef}
                style={{ minHeight, maxHeight, height: calculatedHeight }}
                className="overflow-auto"
            />

            {/* Error banner (JSON mode only) */}
            {isJsonMode && error && (
                <div className="flex items-center gap-2 px-3 py-2 bg-destructive/10 border-t border-destructive/20">
                    <AlertCircle className="h-3.5 w-3.5 text-destructive shrink-0" />
                    <p className="text-xs text-destructive">{error}</p>
                </div>
            )}
        </div>
    )
}
