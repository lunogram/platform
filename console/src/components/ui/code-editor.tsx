import { useEffect, useRef, useState, useMemo, useCallback } from "react"
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
import {
    autocompletion,
    completionKeymap,
    type CompletionContext,
    type CompletionResult,
} from "@codemirror/autocomplete"
import { githubLight } from "@fsegurai/codemirror-theme-github-light"
import { githubDark } from "@fsegurai/codemirror-theme-github-dark"
import { Copy, Check, WandSparkles } from "lucide-react"
import { cn } from "@/utils"

interface CodeEditorProps {
    value: string
    onChange: (value: string) => void
    className?: string
    minHeight?: number
    maxHeight?: number
    readOnly?: boolean
    placeholder?: string
    variableNames?: string[]
}

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

export function CodeEditor({
    value,
    onChange,
    className,
    minHeight = 100,
    maxHeight = 400,
    readOnly = false,
    placeholder,
    variableNames,
}: CodeEditorProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const viewRef = useRef<EditorView | null>(null)
    const [copied, setCopied] = useState(false)
    const [isDark, setIsDark] = useState(() => document.documentElement.classList.contains("dark"))

    const themeCompartment = useMemo(() => new Compartment(), [])
    const editableCompartment = useMemo(() => new Compartment(), [])
    const languageCompartment = useMemo(() => new Compartment(), [])
    const completionCompartment = useMemo(() => new Compartment(), [])

    const onChangeRef = useRef(onChange)
    useEffect(() => {
        onChangeRef.current = onChange
    }, [onChange])

    // Keep variable names in a ref so the completion source always has the latest
    const variableNamesRef = useRef(variableNames)
    useEffect(() => {
        variableNamesRef.current = variableNames
    }, [variableNames])

    // Build a CodeMirror completion source for {{ variable }} patterns
    const variableCompletionSource = useCallback(
        (ctx: CompletionContext): CompletionResult | null => {
            const names = variableNamesRef.current
            if (!names || names.length === 0) return null

            const pos = ctx.pos
            const line = ctx.state.doc.lineAt(pos)
            const textBefore = line.text.slice(0, pos - line.from)

            // Find the last `{{` before cursor with no closing `}}`
            const lastBrace = textBefore.lastIndexOf("{{")
            if (lastBrace === -1) return null

            const between = textBefore.slice(lastBrace + 2)
            if (between.includes("}}")) return null

            // The filter text is everything after `{{` (trimmed)
            const filterText = between.trimStart()
            const from = line.from + lastBrace + 2 + (between.length - between.trimStart().length)

            return {
                from,
                options: names.map((name) => ({
                    label: name,
                    apply: ` ${name} }}`,
                    type: "variable",
                })),
                filter: true,
            }
        },
        [],
    )

    // Track dark mode
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

    // Initialize editor
    useEffect(() => {
        if (!containerRef.current) return

        const updateListener = EditorView.updateListener.of((update) => {
            if (update.docChanged) {
                const doc = update.state.doc.toString()
                onChangeRef.current(doc)
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
            completionCompartment.of(
                variableNames && variableNames.length > 0
                    ? [autocompletion({ override: [variableCompletionSource] })]
                    : [],
            ),
            languageCompartment.of(isJsonContent(value) ? json() : []),
            keymap.of([...defaultKeymap, ...historyKeymap, ...foldKeymap, ...completionKeymap]),
            baseTheme,
            themeCompartment.of(isDark ? githubDark : githubLight),
            editableCompartment.of(EditorView.editable.of(!readOnly)),
            ...(readOnly ? [EditorView.lineWrapping] : []),
            updateListener,
            ...(placeholder
                ? [EditorView.contentAttributes.of({ "aria-placeholder": placeholder })]
                : []),
            EditorView.contentAttributes.of({ "aria-label": "Code Editor" }),
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

        return () => {
            view.destroy()
            viewRef.current = null
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [themeCompartment, editableCompartment, languageCompartment, completionCompartment])

    // Update theme
    useEffect(() => {
        const view = viewRef.current
        if (!view) return
        view.dispatch({
            effects: themeCompartment.reconfigure(isDark ? githubDark : githubLight),
        })
    }, [isDark, themeCompartment])

    // Update editable
    useEffect(() => {
        const view = viewRef.current
        if (!view) return
        view.dispatch({
            effects: editableCompartment.reconfigure(EditorView.editable.of(!readOnly)),
        })
    }, [readOnly, editableCompartment])

    // Update variable completions
    useEffect(() => {
        const view = viewRef.current
        if (!view) return
        view.dispatch({
            effects: completionCompartment.reconfigure(
                variableNames && variableNames.length > 0
                    ? [autocompletion({ override: [variableCompletionSource] })]
                    : [],
            ),
        })
    }, [variableNames, completionCompartment, variableCompletionSource])

    // Sync external value & auto-detect language
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

        // Update language mode based on content
        view.dispatch({
            effects: languageCompartment.reconfigure(isJsonContent(value) ? json() : []),
        })
    }, [value, languageCompartment])

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
            // Not valid JSON, nothing to format
        }
    }

    const lineCount = value.split("\n").length
    const lineHeight = 20
    const padding = 24
    const calculatedHeight = Math.min(
        Math.max(lineCount * lineHeight + padding, minHeight),
        maxHeight,
    )
    const showFormat = canFormatAsJson(value)

    return (
        <div className={cn("relative rounded-md border", className)}>
            {/* Toolbar */}
            <div className="absolute top-2 right-2 z-10 flex items-center gap-1">
                {showFormat && (
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
        </div>
    )
}
