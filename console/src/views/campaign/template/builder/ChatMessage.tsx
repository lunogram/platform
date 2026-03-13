import { memo, useCallback, useEffect, useRef, useState } from "react"
import "./ChatMessage.css"
import Markdown from "react-markdown"
import { Editor, type OnMount } from "@monaco-editor/react"
import type { editor } from "monaco-editor"
import {
    ImageIcon,
    Crosshair,
    Loader2,
    Code2,
    FileCode2,
    Copy,
    Check,
    CheckCircle2,
    AlertCircle,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import type { BuilderMessage, StreamingSegment, CompileCheckStatus } from "./types"

/**
 * Strip trailing partial fence markers from streaming text content.
 *
 * When a fence opening marker (e.g. "```tsx" or "```tsx{10,20}") is split
 * across chunk boundaries, the incomplete marker leaks into a FenceText event
 * and gets accumulated into the text segment. CommonMark treats an unclosed
 * fence as an open code block extending to end-of-document, rendering an empty
 * <pre><code>. This function removes any trailing partial marker so
 * react-markdown never sees it.
 *
 * The pattern matches a trailing sequence of 1+ backticks optionally followed
 * by a partial language tag (word chars) and an optional partial range
 * annotation (e.g. "{10,20}" or "{10," or "{1"), anchored at end-of-string.
 * Safe during streaming because the next event will either complete the fence
 * (creating a code segment) or append more text.
 */
function stripTrailingFenceMarker(text: string): string {
    return text.replace(/`{1,}[\w]*(?:\{[\d,]*\}?)?\s*$/, "")
}

/**
 * Strip range annotations from code fence language identifiers.
 *
 * The AI uses ```tsx{147,191} to indicate range edits. This syntax is
 * processed by the backend fence parser during streaming, but when messages
 * are loaded from the database the raw markdown is passed directly to
 * react-markdown. CommonMark doesn't understand range annotations — the
 * curly braces break the info string parsing, producing an empty
 * <pre><code class="language-tsx{147,191"> block with no content.
 *
 * This function converts e.g. ```tsx{147,191} → ```tsx so react-markdown
 * renders the code block correctly.
 */
function stripFenceRangeAnnotations(text: string): string {
    return text.replace(/(```(?:tsx|jsx|html))\{\d+,\d+\}/g, "$1")
}

/**
 * Extract the target filename from code content.
 *
 * Always returns "template.tsx" since we only produce full templates.
 */
function extractFilename(): string {
    return "template.tsx"
}

/** Small header bar showing the file being edited */
function FileHeader({ filename }: { filename: string }) {
    return (
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted/80 border-b text-xs text-muted-foreground">
            <FileCode2 className="h-3.5 w-3.5 shrink-0" />
            <span className="font-medium truncate">{filename}</span>
        </div>
    )
}

/** Small copy-to-clipboard button (matches json-view.tsx pattern) */
function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    useEffect(() => {
        return () => {
            if (timerRef.current) clearTimeout(timerRef.current)
        }
    }, [])

    const handleCopy = useCallback(async () => {
        await navigator.clipboard.writeText(text)
        setCopied(true)
        if (timerRef.current) clearTimeout(timerRef.current)
        timerRef.current = setTimeout(() => setCopied(false), 2000)
    }, [text])

    return (
        <button
            type="button"
            onClick={handleCopy}
            className="absolute top-1.5 right-1.5 z-10 p-1 rounded-md text-muted-foreground/60 hover:bg-muted hover:text-foreground transition-colors cursor-pointer opacity-0 group-hover/code:opacity-100"
            title="Copy to clipboard"
        >
            {copied ? (
                <Check className="h-3.5 w-3.5 text-green-500" />
            ) : (
                <Copy className="h-3.5 w-3.5" />
            )}
        </button>
    )
}

/** Shared Monaco options for read-only code blocks */
const CODE_BLOCK_OPTIONS = {
    readOnly: true,
    domReadOnly: true,
    minimap: { enabled: false },
    fontSize: 12,
    lineHeight: 18,
    scrollBeyondLastLine: false,
    padding: { top: 8, bottom: 8 },
    renderLineHighlight: "none" as const,
    lineNumbers: "off" as const,
    glyphMargin: false,
    folding: false,
    scrollbar: { vertical: "auto" as const, horizontal: "auto" as const },
    wordWrap: "on" as const,
    overviewRulerLanes: 0,
    hideCursorInOverviewRuler: true,
    overviewRulerBorder: false,
    contextmenu: false,
    guides: { indentation: false },
    stickyScroll: { enabled: false },
    tabSize: 2,
    automaticLayout: true,
} as const

/** Animated label for in-progress code streaming — cycles dots to indicate activity */
function StreamingLabel({ text }: { text: string }) {
    const [dotCount, setDotCount] = useState(1)

    useEffect(() => {
        const interval = setInterval(() => {
            setDotCount((prev) => (prev % 3) + 1)
        }, 400)
        return () => clearInterval(interval)
    }, [])

    return (
        <span className="inline-flex items-center gap-1.5">
            <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500" />
            </span>
            <span>
                {text}
                {".".repeat(dotCount)}
                <span className="invisible">{".".repeat(3 - dotCount)}</span>
            </span>
        </span>
    )
}

/**
 * Monaco code card — used for both live-streaming and finalized code blocks.
 *
 * When `streaming` is true (and `done` is false), the label animates to
 * indicate activity and the editor auto-scrolls on every code update.
 * Otherwise it renders a static "done" label.
 */
function CodeCard({
    code,
    mode,
    description,
    startLine,
    endLine,
    streaming,
    done,
}: {
    code: string
    mode?: string
    description?: string
    startLine?: number
    endLine?: number
    /** Whether this card is part of an active stream */
    streaming?: boolean
    /** Whether the code block has finished streaming (only relevant when streaming) */
    done?: boolean
}) {
    const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null)
    const lineCount = code.split("\n").length
    const height = Math.min(lineCount * 18 + 16, 240)
    const filename = extractFilename()
    const isRange = mode === "range" && startLine != null && endLine != null
    const isActive = streaming && !done

    const doneLabel =
        description ?? (isRange ? `Lines ${startLine}–${endLine} edited` : "Code applied")
    const activeLabel =
        description ?? (isRange ? `Editing lines ${startLine}–${endLine}` : "Writing code")

    const handleMount: OnMount = (ed) => {
        editorRef.current = ed
        const model = ed.getModel()
        if (model) {
            ed.revealLine(model.getLineCount())
        }
    }

    // Auto-scroll to bottom while streaming
    useEffect(() => {
        if (!isActive) return
        const ed = editorRef.current
        if (!ed) return
        const model = ed.getModel()
        if (model) {
            ed.revealLine(model.getLineCount())
        }
    }, [code, isActive])

    return (
        <div className="mt-1.5 min-w-[60%]">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground py-1">
                {isActive ? (
                    <StreamingLabel text={activeLabel} />
                ) : (
                    <>
                        <Code2 className="h-3.5 w-3.5" />
                        <span>{doneLabel}</span>
                    </>
                )}
            </div>
            <div className="rounded-lg border bg-muted/50 overflow-hidden mt-1 relative group/code">
                <CopyButton text={code} />
                <FileHeader filename={filename} />
                <div style={{ height }}>
                    <Editor
                        value={code}
                        defaultLanguage="typescript"
                        options={CODE_BLOCK_OPTIONS}
                        onMount={handleMount}
                    />
                </div>
            </div>
        </div>
    )
}

interface ChatMessageProps {
    message: BuilderMessage
}

export const ChatMessage = memo(function ChatMessage({ message }: ChatMessageProps) {
    const isAgent = message.role === "agent"
    const isSystem = message.role === "system"

    if (isSystem) {
        return (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground/60 py-0.5">
                <AlertCircle className="h-3 w-3 shrink-0" />
                <span>Compile error — auto-fixing…</span>
            </div>
        )
    }

    return (
        <div className={`flex ${isAgent ? "" : "justify-end"}`}>
            <div className={`flex flex-col gap-1.5 max-w-[85%] ${isAgent ? "" : "items-end"}`}>
                {/* Context pills on user messages */}
                {!isAgent && (message.images?.length || message.sections?.length) && (
                    <div className="flex flex-wrap gap-1.5">
                        {message.images?.map((img) => (
                            <Tooltip key={img.id}>
                                <TooltipTrigger asChild>
                                    <Badge
                                        variant="secondary"
                                        className="gap-1 font-normal max-w-full bg-primary/10 text-primary border-primary/20"
                                    >
                                        <ImageIcon className="h-3 w-3 shrink-0" />
                                        <span className="truncate">{img.name}</span>
                                    </Badge>
                                </TooltipTrigger>
                                <TooltipContent side="top" className="p-1">
                                    <img
                                        src={img.url}
                                        alt={img.name}
                                        className="rounded max-w-48 max-h-32 object-cover"
                                    />
                                </TooltipContent>
                            </Tooltip>
                        ))}
                        {message.sections?.map((section) => (
                            <Badge
                                key={section.label}
                                variant="secondary"
                                className="gap-1 font-normal max-w-full bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20"
                            >
                                <Crosshair className="h-3 w-3 shrink-0" />
                                <span className="truncate">
                                    {section.label.length > 15
                                        ? section.label.slice(0, 15) + "…"
                                        : section.label}
                                </span>
                            </Badge>
                        ))}
                    </div>
                )}

                {/* Message content */}
                {isAgent ? (
                    message.segments && message.segments.length > 0 ? (
                        // Render segments in the order they arrived during streaming.
                        <div className="flex flex-col gap-1.5">
                            {message.segments.map((seg, i) => {
                                if (seg.type === "text") {
                                    return (
                                        <div
                                            key={`text-${i}`}
                                            className="text-sm leading-relaxed text-foreground builder-markdown"
                                        >
                                            <Markdown>
                                                {stripFenceRangeAnnotations(seg.content)}
                                            </Markdown>
                                        </div>
                                    )
                                }
                                return (
                                    <CodeCard
                                        key={`code-${i}`}
                                        code={seg.content}
                                        mode={seg.mode}
                                        description={seg.description}
                                        startLine={seg.startLine}
                                        endLine={seg.endLine}
                                    />
                                )
                            })}
                        </div>
                    ) : (
                        // Fallback for messages without segments (e.g. loaded from history)
                        <>
                            <div className="text-sm leading-relaxed text-foreground builder-markdown">
                                <Markdown>{stripFenceRangeAnnotations(message.text)}</Markdown>
                            </div>
                            {message.templateSource && <CodeCard code={message.templateSource} />}
                        </>
                    )
                ) : (
                    <div className="rounded-xl px-4 py-2.5 text-sm leading-relaxed bg-primary text-primary-foreground">
                        {message.text}
                    </div>
                )}
            </div>
        </div>
    )
})

/** Typing indicator shown while the agent is generating */
export function TypingIndicator() {
    return (
        <div className="flex">
            <div className="flex items-center gap-1 py-2">
                <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/50 animate-bounce [animation-delay:0ms]" />
                <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/50 animate-bounce [animation-delay:150ms]" />
                <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/50 animate-bounce [animation-delay:300ms]" />
            </div>
        </div>
    )
}

/** Subtle inline indicator for compile check status */
export function CompileCheckIndicator({
    status,
    error,
    attempt,
}: {
    status: CompileCheckStatus
    error: string | null
    attempt: number
}) {
    if (status === "idle") return null

    if (status === "checking") {
        return (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground/60 py-0.5">
                <Loader2 className="h-3 w-3 animate-spin" />
                <span>Verifying…</span>
            </div>
        )
    }

    if (status === "success") {
        return (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground/60 py-0.5">
                <CheckCircle2 className="h-3 w-3 text-green-500/70" />
                <span>Compiled</span>
            </div>
        )
    }

    // status === "error"
    return (
        <div className="flex items-center gap-1.5 text-xs text-destructive/70 py-0.5">
            <AlertCircle className="h-3 w-3 shrink-0" />
            <span className="truncate">{attempt < 3 ? "Fixing…" : "Compile error"}</span>
        </div>
    )
}

export function AgentWritingIndicator({
    segments,
    isTyping,
}: {
    segments: StreamingSegment[]
    isTyping: boolean
}) {
    // If no segments have arrived yet and still typing, show the thinking spinner
    if (segments.length === 0 && isTyping) {
        return (
            <div className="flex items-center gap-2 text-sm text-muted-foreground py-1">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>Thinking…</span>
            </div>
        )
    }

    // If no segments and not typing, nothing to show
    if (segments.length === 0) return null

    return (
        <div className="flex flex-col gap-1.5 max-w-[85%]">
            {segments.map((seg, i) => {
                if (seg.type === "text") {
                    if (!seg.content) return null
                    return (
                        <div
                            key={`text-${i}`}
                            className="text-sm leading-relaxed text-foreground builder-markdown"
                        >
                            <Markdown>
                                {stripFenceRangeAnnotations(stripTrailingFenceMarker(seg.content))}
                            </Markdown>
                        </div>
                    )
                }
                // seg.type === "code" — skip rendering until first code token arrives
                if (!seg.content) return null
                return (
                    <CodeCard
                        key={`code-${i}`}
                        code={seg.content}
                        streaming
                        done={seg.done}
                        mode={seg.mode}
                        description={seg.description}
                        startLine={seg.startLine}
                        endLine={seg.endLine}
                    />
                )
            })}
        </div>
    )
}
