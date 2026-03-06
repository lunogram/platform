import { useEffect, useRef, useState, useCallback, useMemo } from "react"
import { EditorState, Compartment } from "@codemirror/state"
import {
    EditorView,
    keymap,
    lineNumbers,
    highlightActiveLine,
    highlightActiveLineGutter,
} from "@codemirror/view"
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands"
import { json } from "@codemirror/lang-json"
import { foldGutter, indentOnInput, bracketMatching, foldKeymap } from "@codemirror/language"
import { linter, lintGutter } from "@codemirror/lint"
import type { Diagnostic } from "@codemirror/lint"
import { autocompletion, completionKeymap } from "@codemirror/autocomplete"
import { githubLight } from "@fsegurai/codemirror-theme-github-light"
import { githubDark } from "@fsegurai/codemirror-theme-github-dark"
import { Copy, Check, WandSparkles, AlertCircle } from "lucide-react"
import { cn } from "@/utils"

interface JsonEditorProps {
    value: string
    onChange: (value: string) => void
    onError?: (error: string | null) => void
    className?: string
    minHeight?: number
    maxHeight?: number
    readOnly?: boolean
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
})

// Custom JSON linter that validates both syntax and structure
function createJsonLinter() {
    return linter((view) => {
        const diagnostics: Diagnostic[] = []
        const doc = view.state.doc.toString()

        if (!doc.trim()) return diagnostics

        try {
            const parsed = JSON.parse(doc)
            if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
                diagnostics.push({
                    from: 0,
                    to: doc.length,
                    severity: "error",
                    message: "Root must be a JSON object",
                })
            }
        } catch (e) {
            // Parse the error to find position
            const message = e instanceof Error ? e.message : "Invalid JSON"
            // Try to extract position from error message (e.g., "at position 42")
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

export function JsonEditor({
    value,
    onChange,
    onError,
    className,
    minHeight = 100,
    maxHeight = 400,
    readOnly = false,
}: JsonEditorProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const viewRef = useRef<EditorView | null>(null)
    const [copied, setCopied] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [isDark, setIsDark] = useState(() => document.documentElement.classList.contains("dark"))

    // Create compartments once per component instance
    const themeCompartment = useMemo(() => new Compartment(), [])
    const editableCompartment = useMemo(() => new Compartment(), [])

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

    // Validate JSON and report errors
    const validateJson = useCallback(
        (doc: string) => {
            if (!doc.trim()) {
                setError(null)
                onError?.(null)
                return
            }

            try {
                const parsed = JSON.parse(doc)
                if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
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
        [onError],
    )

    // Store callbacks in refs to avoid recreating the editor
    const onChangeRef = useRef(onChange)
    const validateJsonRef = useRef(validateJson)

    useEffect(() => {
        onChangeRef.current = onChange
        validateJsonRef.current = validateJson
    }, [onChange, validateJson])

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
            json(),
            createJsonLinter(),
            lintGutter(),
            keymap.of([...defaultKeymap, ...historyKeymap, ...foldKeymap, ...completionKeymap]),
            baseTheme,
            themeCompartment.of(isDark ? githubDark : githubLight),
            editableCompartment.of(EditorView.editable.of(!readOnly)),
            updateListener,
            EditorView.contentAttributes.of({ "aria-label": "JSON Editor" }),
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
        validateJson(value)

        return () => {
            view.destroy()
            viewRef.current = null
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [themeCompartment, editableCompartment]) // Recreate if compartments change

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

    // Sync external value changes
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
    }, [value])

    const handleCopy = async () => {
        await navigator.clipboard.writeText(value)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    const handleFormat = () => {
        try {
            const parsed = JSON.parse(value)
            const formatted = JSON.stringify(parsed, null, 2)
            onChange(formatted)

            const view = viewRef.current
            if (view) {
                view.dispatch({
                    changes: {
                        from: 0,
                        to: view.state.doc.length,
                        insert: formatted,
                    },
                })
            }
        } catch {
            // Can't format invalid JSON
        }
    }

    // Calculate dynamic height based on line count
    const lineCount = value.split("\n").length
    const lineHeight = 20 // approximately
    const padding = 24
    const calculatedHeight = Math.min(
        Math.max(lineCount * lineHeight + padding, minHeight),
        maxHeight,
    )

    return (
        <div className={cn("relative", className)}>
            {/* Toolbar */}
            <div className="absolute top-2 right-2 z-10 flex items-center gap-1">
                <button
                    onClick={handleFormat}
                    disabled={!!error}
                    className={cn(
                        "p-1.5 rounded-md text-muted-foreground transition-colors",
                        error
                            ? "opacity-50 cursor-not-allowed"
                            : "hover:bg-muted hover:text-foreground cursor-pointer",
                    )}
                    title="Format JSON"
                >
                    <WandSparkles className="h-3.5 w-3.5" />
                </button>
                <button
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

            {/* Error banner */}
            {error && (
                <div className="flex items-center gap-2 px-3 py-2 bg-destructive/10 border-t border-destructive/20">
                    <AlertCircle className="h-3.5 w-3.5 text-destructive shrink-0" />
                    <p className="text-xs text-destructive">{error}</p>
                </div>
            )}
        </div>
    )
}
