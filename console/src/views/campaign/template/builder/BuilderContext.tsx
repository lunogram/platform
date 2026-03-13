import { useReducer, useCallback, useRef, useEffect, useMemo, type ReactNode } from "react"
import { createUuid } from "@/utils"
import { backofficeClient } from "@/oapi/backoffice-client"
import { fetchSSE } from "@/lib/sse"
import {
    BuilderActionType,
    type BuilderState,
    type BuilderAction,
    type BuilderMessage,
    type FinalizedSegment,
    type SelectedSection,
    type ViewportMode,
    type CompileCheckStatus,
} from "./types"
import type { Image } from "@/types"
import type { UUID } from "@/types/common"
import {
    BuilderActionsContext,
    BuilderThreadContext,
    BuilderStreamContext,
    type BuilderActionsContextValue,
    type BuilderThreadContextValue,
    type BuilderStreamContextValue,
} from "./BuilderContextDef"

const MAX_COMPILE_FIX_ATTEMPTS = 3

const initialState: BuilderState = {
    messages: [],
    currentSource: null,
    selectedImages: [],
    selectedSections: [],
    isAgentTyping: false,
    versionCount: 0,
    viewport: "desktop",
    conversationId: null,
    streamingSegments: [],
    streamError: null,
    compileCheckStatus: "idle",
    compileCheckError: null,
    compileFixAttempts: 0,
}

function builderReducer(state: BuilderState, action: BuilderAction): BuilderState {
    switch (action.type) {
        case BuilderActionType.SEND_MESSAGE: {
            const userMessage: BuilderMessage = {
                id: createUuid(),
                role: action.isAutoFix ? "system" : "user",
                kind: action.isAutoFix ? "compile_error" : undefined,
                text: action.text,
                images: action.images.length > 0 ? [...action.images] : undefined,
                sections: action.sections.length > 0 ? [...action.sections] : undefined,
                createdAt: new Date(),
            }
            return {
                ...state,
                messages: [...state.messages, userMessage],
                // Clear context after sending
                selectedImages: [],
                selectedSections: [],
                isAgentTyping: true,
                streamingSegments: [],
                // Reset compile-fix attempts only for user-initiated messages
                // (auto-fix messages must preserve the counter to enforce the retry budget)
                compileFixAttempts: action.isAutoFix ? state.compileFixAttempts : 0,
                compileCheckStatus: "idle" as CompileCheckStatus,
                compileCheckError: null,
            }
        }
        case BuilderActionType.SET_AGENT_TYPING:
            return { ...state, isAgentTyping: action.isTyping }
        case BuilderActionType.SET_CONVERSATION_ID:
            return { ...state, conversationId: action.conversationId }
        case BuilderActionType.SELECT_IMAGE:
            if (state.selectedImages.some((img) => img.id === action.image.id)) {
                return state
            }
            return { ...state, selectedImages: [...state.selectedImages, action.image] }
        case BuilderActionType.DESELECT_IMAGE:
            return {
                ...state,
                selectedImages: state.selectedImages.filter((img) => img.id !== action.imageId),
            }
        case BuilderActionType.SET_IMAGES:
            return { ...state, selectedImages: action.images }
        case BuilderActionType.TOGGLE_SECTION: {
            const exists = state.selectedSections.some((s) => s.label === action.section.label)
            if (exists) {
                return {
                    ...state,
                    selectedSections: state.selectedSections.filter(
                        (s) => s.label !== action.section.label,
                    ),
                }
            }
            return { ...state, selectedSections: [...state.selectedSections, action.section] }
        }
        case BuilderActionType.DESELECT_SECTION:
            return {
                ...state,
                selectedSections: state.selectedSections.filter((s) => s.label !== action.label),
            }
        case BuilderActionType.CLEAR_SECTIONS:
            return { ...state, selectedSections: [] }
        case BuilderActionType.SET_VIEWPORT:
            return { ...state, viewport: action.viewport }
        case BuilderActionType.LOAD_TEMPLATE:
            return { ...state, currentSource: action.source }

        case BuilderActionType.STREAM_START:
            return {
                ...state,
                isAgentTyping: true,
                streamingSegments: [],
                streamError: null,
                conversationId: action.conversationId,
                compileCheckStatus: "idle" as CompileCheckStatus,
                compileCheckError: null,
            }

        case BuilderActionType.STREAM_DELTA: {
            // Append to the last text segment, or create a new one.
            // Because text and code events are mutually exclusive from
            // the backend, a delta event can never arrive while a code
            // block is in progress.
            const segs = [...state.streamingSegments]
            const last = segs[segs.length - 1]
            if (last && last.type === "text") {
                segs[segs.length - 1] = { ...last, content: last.content + action.content }
            } else {
                segs.push({ type: "text", content: action.content })
            }
            return { ...state, streamingSegments: segs }
        }

        case BuilderActionType.STREAM_CODE_START: {
            const segs = [...state.streamingSegments]
            segs.push({
                type: "code",
                content: "",
                mode: action.mode ?? "write",
                description: action.description,
                startLine: action.startLine,
                endLine: action.endLine,
                done: false,
            })
            return {
                ...state,
                streamingSegments: segs,
            }
        }

        case BuilderActionType.STREAM_CODE_DELTA: {
            // Always append to the last segment — guaranteed to be an
            // in-progress code segment because the backend never
            // interleaves text events inside a fenced block.
            const segs = [...state.streamingSegments]
            const last = segs[segs.length - 1]
            if (last && last.type === "code" && !last.done) {
                segs[segs.length - 1] = { ...last, content: last.content + action.content }
            }
            return {
                ...state,
                streamingSegments: segs,
            }
        }

        case BuilderActionType.STREAM_CODE_END: {
            // Extract the accumulated code content from the last streaming segment.
            const segs = [...state.streamingSegments]
            const last = segs[segs.length - 1]
            let newSource = state.currentSource
            if (last && last.type === "code" && !last.done) {
                if (
                    last.mode === "range" &&
                    last.startLine != null &&
                    last.endLine != null &&
                    state.currentSource
                ) {
                    // Splice: replace lines startLine..endLine (1-indexed, inclusive)
                    // with the new code block content. Falls back to full replacement
                    // if the range is invalid.
                    const lines = state.currentSource.split("\n")
                    const start = last.startLine - 1 // convert to 0-indexed
                    const end = last.endLine // endLine is inclusive, so splice end is exclusive at endLine

                    if (
                        start >= 0 &&
                        start < lines.length &&
                        end >= last.startLine &&
                        end <= lines.length
                    ) {
                        // Strip trailing newline from streamed content to avoid
                        // inserting an extra blank line — LLM outputs typically
                        // include a trailing newline that is not part of the
                        // intended replacement.
                        const trimmedContent = last.content.endsWith("\n")
                            ? last.content.slice(0, -1)
                            : last.content
                        const replacementLines = trimmedContent.split("\n")
                        const spliced = [
                            ...lines.slice(0, start),
                            ...replacementLines,
                            ...lines.slice(end),
                        ]
                        newSource = spliced.join("\n")
                    } else {
                        // Invalid range — fall back to full replacement
                        newSource = last.content
                    }
                } else {
                    // Full replacement (default)
                    newSource = last.content
                }
                segs[segs.length - 1] = { ...last, done: true }
            }
            return {
                ...state,
                currentSource: newSource,
                versionCount: action.versionNumber,
                streamingSegments: segs,
            }
        }

        case BuilderActionType.STREAM_DONE: {
            // Snapshot streaming segments into a pending agent message.
            // Promote the agent message into messages[] immediately so
            // the transition from streaming → finalized happens now
            // (while the user is still reading) rather than later when
            // they send a follow-up (causing a jarring re-render).
            const finalSegments: FinalizedSegment[] = state.streamingSegments
                .filter((seg) => seg.content.length > 0)
                .map((seg) =>
                    seg.type === "text"
                        ? { type: "text" as const, content: seg.content }
                        : {
                              type: "code" as const,
                              content: seg.content,
                              mode: seg.mode,
                              description: seg.description,
                              startLine: seg.startLine,
                              endLine: seg.endLine,
                          },
                )
            const agentMessage: BuilderMessage = {
                id: action.message.id,
                role: "agent",
                text: action.message.content,
                templateSource: state.currentSource ?? undefined,
                version: state.versionCount,
                segments: finalSegments.length > 0 ? finalSegments : undefined,
                createdAt: new Date(action.message.created_at),
            }
            return {
                ...state,
                messages: [...state.messages, agentMessage],
                isAgentTyping: false,
                streamingSegments: [],
            }
        }

        case BuilderActionType.STREAM_ERROR:
            return {
                ...state,
                isAgentTyping: false,
                streamingSegments: [],
                streamError: action.error,
            }

        case BuilderActionType.COMPILE_CHECK_START:
            return {
                ...state,
                compileCheckStatus: "checking" as CompileCheckStatus,
                compileCheckError: null,
            }

        case BuilderActionType.COMPILE_CHECK_SUCCESS:
            return {
                ...state,
                compileCheckStatus: "success" as CompileCheckStatus,
                compileCheckError: null,
            }

        case BuilderActionType.COMPILE_CHECK_ERROR:
            return {
                ...state,
                compileCheckStatus: "error" as CompileCheckStatus,
                compileCheckError: action.error,
                compileFixAttempts: state.compileFixAttempts + 1,
            }

        default:
            return state
    }
}

interface BuilderProviderProps {
    projectId: string
    templateId: string
    children: ReactNode
}

/**
 * Ensures a backoffice conversation exists, creating one lazily on
 * first message. Returns the conversation ID.
 */
async function ensureConversation(
    conversationIdRef: React.RefObject<string | null>,
    dispatch: React.Dispatch<BuilderAction>,
    projectId: string,
    templateId: string,
): Promise<string> {
    if (conversationIdRef.current) {
        return conversationIdRef.current
    }

    const { data, error } = await backofficeClient.POST("/v1/conversations", {
        headers: { "X-Project-ID": projectId },
        body: { template_id: templateId },
    })

    if (error || !data) {
        throw new Error("Failed to create conversation")
    }

    conversationIdRef.current = data.id
    dispatch({ type: BuilderActionType.SET_CONVERSATION_ID, conversationId: data.id })
    return data.id
}

export function BuilderProvider({ projectId, templateId, children }: BuilderProviderProps) {
    const [state, dispatch] = useReducer(builderReducer, initialState)

    // Use a ref for conversationId so the sendMessage callback doesn't
    // need to re-create on every state change — avoids stale closure issues.
    const conversationIdRef = useRef<string | null>(null)

    // Keep refs for mutable state that sendMessage needs, so the callback
    // stays referentially stable and never closes over stale values.
    const selectedImagesRef = useRef(state.selectedImages)
    const selectedSectionsRef = useRef(state.selectedSections)
    const currentSourceRef = useRef(state.currentSource)
    const stateRef = useRef(state)

    useEffect(() => {
        selectedImagesRef.current = state.selectedImages
    }, [state.selectedImages])
    useEffect(() => {
        selectedSectionsRef.current = state.selectedSections
    }, [state.selectedSections])
    useEffect(() => {
        currentSourceRef.current = state.currentSource
    }, [state.currentSource])
    useEffect(() => {
        stateRef.current = state
    }, [state])

    // Abort controller for cancelling in-flight SSE streams
    const abortRef = useRef<AbortController | null>(null)

    // --------------- Code-delta chunker ---------------
    // Buffers all code_delta payloads and drips them into the reducer
    // line-by-line using requestAnimationFrame so the Monaco editor
    // shows a smooth streaming animation. This is essential because
    // backends often deliver code in large chunks (or even all at once
    // for tool calls from models like Mistral), which would otherwise
    // cause the code block to appear fully populated in a single frame.
    const chunkerRef = useRef<{
        /** Lines waiting to be dispatched */
        buffer: string[]
        /** Whether a rAF loop is scheduled */
        scheduled: boolean
        /** rAF handle for cleanup */
        rafId: number | null
        /** Resolve callback for chunkerDrain promise */
        drainResolve: (() => void) | null
    }>({
        buffer: [],
        scheduled: false,
        rafId: null,
        drainResolve: null,
    })

    /** Number of lines to dispatch per animation frame */
    const LINES_PER_FRAME = 3

    /** Push content into the chunker buffer and start draining */
    const chunkerPush = useCallback(
        (content: string) => {
            const ch = chunkerRef.current
            // Split on newlines, preserving line endings so the reassembled
            // code is byte-identical to the original.
            const lines = content.split("\n")
            for (let i = 0; i < lines.length; i++) {
                // Re-attach the newline to every line except the last
                ch.buffer.push(i < lines.length - 1 ? lines[i] + "\n" : lines[i])
            }

            if (!ch.scheduled) {
                ch.scheduled = true
                const drain = () => {
                    const batch = ch.buffer.splice(0, LINES_PER_FRAME)
                    if (batch.length > 0) {
                        dispatch({
                            type: BuilderActionType.STREAM_CODE_DELTA,
                            content: batch.join(""),
                        })
                    }
                    if (ch.buffer.length > 0) {
                        ch.rafId = requestAnimationFrame(drain)
                    } else {
                        ch.scheduled = false
                        ch.rafId = null
                        // Resolve any pending drain promise (from chunkerDrain)
                        ch.drainResolve?.()
                        ch.drainResolve = null
                    }
                }
                ch.rafId = requestAnimationFrame(drain)
            }
        },
        [dispatch],
    )

    /** Wait for the chunker to finish draining all buffered content
     *  via the rAF animation loop. Returns immediately if the buffer
     *  is already empty. This preserves the line-by-line animation
     *  instead of dumping everything in a single frame. */
    const chunkerDrain = useCallback((): Promise<void> => {
        const ch = chunkerRef.current
        if (ch.buffer.length === 0 && !ch.scheduled) {
            return Promise.resolve()
        }
        return new Promise<void>((resolve) => {
            // Resolve any previously pending drain promise so it doesn't
            // hang forever if chunkerDrain is called more than once before
            // the buffer empties.
            ch.drainResolve?.()
            ch.drainResolve = resolve
        })
    }, [])

    /** Cancel all pending chunker work (used on abort / unmount) */
    const chunkerCancel = useCallback(() => {
        const ch = chunkerRef.current
        if (ch.rafId != null) {
            cancelAnimationFrame(ch.rafId)
            ch.rafId = null
        }
        ch.buffer.length = 0
        ch.scheduled = false
        // Resolve any pending drain promise so callers don't hang
        ch.drainResolve?.()
        ch.drainResolve = null
    }, [])

    // Abort on unmount
    useEffect(() => {
        return () => {
            abortRef.current?.abort()
            chunkerCancel()
        }
    }, [chunkerCancel])

    const sendMessage = useCallback(
        async (text: string, options?: { isAutoFix?: boolean }) => {
            const images = [...selectedImagesRef.current]
            const sections = [...selectedSectionsRef.current]

            dispatch({
                type: BuilderActionType.SEND_MESSAGE,
                text,
                images,
                sections,
                isAutoFix: options?.isAutoFix,
            })

            // Abort any in-flight stream before starting a new one
            abortRef.current?.abort()
            chunkerCancel()
            const controller = new AbortController()
            abortRef.current = controller

            try {
                const conversationId = await ensureConversation(
                    conversationIdRef,
                    dispatch,
                    projectId,
                    templateId,
                )

                // Track pending code_end promises so the "done" event
                // can wait for all code animations to finish before
                // finalizing the message (clearing streamingSegments).
                let pendingCodeEnd: Promise<void> | null = null

                await fetchSSE(
                    `/backoffice/v1/conversations/${conversationId}/messages`,
                    {
                        method: "POST",
                        headers: { "X-Project-ID": projectId },
                        signal: controller.signal,
                        body: JSON.stringify({
                            message: text,
                            ...(options?.isAutoFix && {
                                role: "system",
                                kind: "compile_error",
                            }),
                            context: {
                                images: images.map((img) => ({
                                    name: img.name,
                                    url: img.url,
                                })),
                                section:
                                    sections.length > 0
                                        ? {
                                              label: sections[0].label,
                                              html: sections[0].textContent ?? "",
                                          }
                                        : undefined,
                                current_template: currentSourceRef.current,
                            },
                        }),
                    },
                    {
                        onEvent(event, data: Record<string, any>) {
                            // Ignore events after abort
                            if (controller.signal.aborted) return

                            switch (event) {
                                case "message_created":
                                    dispatch({
                                        type: BuilderActionType.STREAM_START,
                                        messageId: data.message_id,
                                        conversationId: data.conversation_id,
                                    })
                                    break
                                case "delta":
                                    dispatch({
                                        type: BuilderActionType.STREAM_DELTA,
                                        content: data.content,
                                    })
                                    break
                                case "code_start":
                                    dispatch({
                                        type: BuilderActionType.STREAM_CODE_START,
                                        mode: data.mode === "range" ? "range" : "write",
                                        description: data.description,
                                        startLine:
                                            data.start_line != null
                                                ? Number(data.start_line)
                                                : undefined,
                                        endLine:
                                            data.end_line != null
                                                ? Number(data.end_line)
                                                : undefined,
                                    })
                                    break
                                case "code_delta":
                                    // Always route through the chunker so the
                                    // Monaco editor shows a smooth line-by-line
                                    // streaming animation — even when the backend
                                    // delivers the entire code block in a single
                                    // SSE event (e.g. Mistral tool calls).
                                    if (
                                        typeof data.content === "string" &&
                                        data.content.length > 0
                                    ) {
                                        chunkerPush(data.content)
                                    }
                                    break
                                case "code_end":
                                    // Wait for the chunker to finish drip-feeding
                                    // lines via rAF so the streaming animation
                                    // plays out before the code segment is marked
                                    // as done. Previously we flushed synchronously,
                                    // which dumped all remaining content in one
                                    // frame — defeating the animation.
                                    pendingCodeEnd = chunkerDrain().then(() => {
                                        // Guard against dispatching after abort
                                        if (controller.signal.aborted) return
                                        dispatch({
                                            type: BuilderActionType.STREAM_CODE_END,
                                            versionId: data.version_id,
                                            versionNumber: data.version_number,
                                        })
                                        pendingCodeEnd = null
                                    })
                                    break
                                case "done":
                                    // Wait for any pending code_end animation to
                                    // complete before finalizing — STREAM_DONE
                                    // clears streamingSegments, so dispatching it
                                    // before STREAM_CODE_END would lose the code.
                                    if (pendingCodeEnd) {
                                        void pendingCodeEnd.then(() => {
                                            if (controller.signal.aborted) return
                                            dispatch({
                                                type: BuilderActionType.STREAM_DONE,
                                                message: data.message,
                                            })
                                        })
                                    } else {
                                        dispatch({
                                            type: BuilderActionType.STREAM_DONE,
                                            message: data.message,
                                        })
                                    }
                                    break
                                case "error":
                                    dispatch({
                                        type: BuilderActionType.STREAM_ERROR,
                                        error: data.error,
                                    })
                                    break
                            }
                        },
                        onError(err) {
                            // Don't dispatch errors for intentional aborts
                            if (controller.signal.aborted) return
                            dispatch({ type: BuilderActionType.STREAM_ERROR, error: err.message })
                        },
                        onClose() {
                            // Stream ended normally — STREAM_DONE should have been dispatched
                        },
                    },
                )
            } catch (err) {
                // Don't dispatch errors for intentional aborts
                if (controller.signal.aborted) return
                dispatch({ type: BuilderActionType.STREAM_ERROR, error: "Failed to send message" })
            }
        },
        [projectId, templateId, chunkerPush, chunkerDrain, chunkerCancel],
    )

    const selectImage = useCallback((image: Image) => {
        dispatch({ type: BuilderActionType.SELECT_IMAGE, image })
    }, [])

    const deselectImage = useCallback((imageId: UUID) => {
        dispatch({ type: BuilderActionType.DESELECT_IMAGE, imageId })
    }, [])

    const setImages = useCallback((images: Image[]) => {
        dispatch({ type: BuilderActionType.SET_IMAGES, images })
    }, [])

    const selectSection = useCallback((section: SelectedSection) => {
        dispatch({ type: BuilderActionType.TOGGLE_SECTION, section })
    }, [])

    const deselectSection = useCallback((label: string) => {
        dispatch({ type: BuilderActionType.DESELECT_SECTION, label })
    }, [])

    const clearSections = useCallback(() => {
        dispatch({ type: BuilderActionType.CLEAR_SECTIONS })
    }, [])

    const setViewport = useCallback((viewport: ViewportMode) => {
        dispatch({ type: BuilderActionType.SET_VIEWPORT, viewport })
    }, [])

    const loadTemplate = useCallback((source: string) => {
        dispatch({ type: BuilderActionType.LOAD_TEMPLATE, source })
    }, [])

    const startCompileCheck = useCallback(() => {
        dispatch({ type: BuilderActionType.COMPILE_CHECK_START })
    }, [])

    // Keep a ref for compileFixAttempts so reportCompileResult doesn't
    // close over stale values.
    const compileFixAttemptsRef = useRef(state.compileFixAttempts)
    useEffect(() => {
        compileFixAttemptsRef.current = state.compileFixAttempts
    }, [state.compileFixAttempts])

    /**
     * Report the compile result of AI-generated template code.
     * On error, if the retry budget hasn't been exhausted, automatically
     * sends a fix request to the AI with enhanced error context including
     * the broken template source around the error location.
     */
    const reportCompileResult = useCallback(
        (result: { success: true } | { success: false; error: string }) => {
            if (result.success) {
                dispatch({ type: BuilderActionType.COMPILE_CHECK_SUCCESS })
                return
            }

            dispatch({ type: BuilderActionType.COMPILE_CHECK_ERROR, error: result.error })

            // Auto-fix: send the error back to the AI if under the retry budget.
            // compileFixAttempts is incremented by the reducer above, so we check
            // the ref value *before* the dispatch landed. The +1 accounts for the
            // increment that just happened.
            if (compileFixAttemptsRef.current + 1 < MAX_COMPILE_FIX_ATTEMPTS) {
                const source = currentSourceRef.current
                let contextSnippet = ""

                if (source) {
                    const lines = source.split("\n")

                    // Try to extract line number from error message (e.g. "Unexpected token (49:9)")
                    const lineMatch = result.error.match(/\((\d+):(\d+)\)/)
                    const errorLine = lineMatch ? parseInt(lineMatch[1], 10) : null

                    // Find the last range edit to show what was spliced
                    const agentMessages = stateRef.current.messages.filter(
                        (m) => m.role === "agent",
                    )
                    const lastMsg =
                        agentMessages.length > 0
                            ? agentMessages[agentMessages.length - 1]
                            : undefined
                    const rangeEdits = lastMsg?.segments?.filter(
                        (s) =>
                            s.type === "code" &&
                            s.mode === "range" &&
                            s.startLine != null &&
                            s.endLine != null,
                    )
                    const lastRangeEdit =
                        rangeEdits && rangeEdits.length > 0
                            ? rangeEdits[rangeEdits.length - 1]
                            : undefined

                    // Build context snippet showing relevant lines
                    const regions: Array<{ label: string; start: number; end: number }> = []

                    if (
                        lastRangeEdit &&
                        lastRangeEdit.type === "code" &&
                        lastRangeEdit.startLine != null &&
                        lastRangeEdit.endLine != null
                    ) {
                        // Show lines around the splice region (5 lines of context on each side)
                        const spliceStart = Math.max(0, lastRangeEdit.startLine - 6)
                        const spliceEnd = Math.min(lines.length, lastRangeEdit.endLine + 5)
                        regions.push({
                            label: `Around splice region (your edit was lines ${lastRangeEdit.startLine}-${lastRangeEdit.endLine})`,
                            start: spliceStart,
                            end: spliceEnd,
                        })
                    }

                    if (errorLine != null) {
                        // Show lines around the reported error location (5 lines of context)
                        const errStart = Math.max(0, errorLine - 6)
                        const errEnd = Math.min(lines.length, errorLine + 5)
                        // Only add if it doesn't overlap significantly with the splice region
                        const overlaps = regions.some((r) => errStart <= r.end && errEnd >= r.start)
                        if (!overlaps) {
                            regions.push({
                                label: `Around error location (line ${errorLine})`,
                                start: errStart,
                                end: errEnd,
                            })
                        } else {
                            // Extend the existing region to cover both
                            const r = regions[0]
                            r.start = Math.min(r.start, errStart)
                            r.end = Math.max(r.end, errEnd)
                            r.label += `, error reported at line ${errorLine}`
                        }
                    }

                    if (regions.length > 0) {
                        const snippets = regions.map((r) => {
                            const regionLines = lines
                                .slice(r.start, r.end)
                                .map((l, i) => `${r.start + i + 1}: ${l}`)
                                .join("\n")
                            return `### ${r.label}\n\`\`\`tsx\n${regionLines}\n\`\`\``
                        })
                        contextSnippet = `\n\n## Current template source (relevant lines)\n${snippets.join("\n\n")}`
                    } else {
                        // Fallback: show first 30 lines for general context
                        const preview = lines
                            .slice(0, 30)
                            .map((l, i) => `${i + 1}: ${l}`)
                            .join("\n")
                        contextSnippet = `\n\n## Current template source (first 30 lines)\n\`\`\`tsx\n${preview}\n\`\`\``
                    }
                }

                const fixPrompt =
                    `The template you generated failed to compile with the following error:\n\n` +
                    `\`\`\`\n${result.error}\n\`\`\`\n` +
                    contextSnippet +
                    `\n\n` +
                    `Please examine the template source above carefully, identify the syntax error, and fix it using the edit_template tool. ` +
                    `Pay close attention to unclosed tags, duplicate elements, or malformed JSX around the indicated lines.`
                void sendMessage(fixPrompt, { isAutoFix: true })
            }
        },
        [sendMessage],
    )

    // ── Split context values ──────────────────────────────────────────

    // Actions context — stable callbacks, never changes identity
    const actionsValue = useMemo<BuilderActionsContextValue>(
        () => ({
            sendMessage,
            selectImage,
            deselectImage,
            setImages,
            selectSection,
            deselectSection,
            clearSections,
            setViewport,
            loadTemplate,
            startCompileCheck,
            reportCompileResult,
        }),
        [
            sendMessage,
            selectImage,
            deselectImage,
            setImages,
            selectSection,
            deselectSection,
            clearSections,
            setViewport,
            loadTemplate,
            startCompileCheck,
            reportCompileResult,
        ],
    )

    // Thread context — only the message list + general state
    // Re-renders subscribers only when messages change (not on streaming deltas)
    const threadValue = useMemo<BuilderThreadContextValue>(
        () => ({
            messages: state.messages,
            currentSource: state.currentSource,
            selectedImages: state.selectedImages,
            selectedSections: state.selectedSections,
            isAgentTyping: state.isAgentTyping,
            versionCount: state.versionCount,
            viewport: state.viewport,
            conversationId: state.conversationId,
            streamError: state.streamError,
        }),
        [
            state.messages,
            state.currentSource,
            state.selectedImages,
            state.selectedSections,
            state.isAgentTyping,
            state.versionCount,
            state.viewport,
            state.conversationId,
            state.streamError,
        ],
    )

    // Stream context — high-frequency streaming data
    const streamValue = useMemo<BuilderStreamContextValue>(
        () => ({
            streamingSegments: state.streamingSegments,
            isAgentTyping: state.isAgentTyping,
            compileCheckStatus: state.compileCheckStatus,
            compileCheckError: state.compileCheckError,
            compileFixAttempts: state.compileFixAttempts,
        }),
        [
            state.streamingSegments,
            state.isAgentTyping,
            state.compileCheckStatus,
            state.compileCheckError,
            state.compileFixAttempts,
        ],
    )

    return (
        <BuilderActionsContext.Provider value={actionsValue}>
            <BuilderThreadContext.Provider value={threadValue}>
                <BuilderStreamContext.Provider value={streamValue}>
                    {children}
                </BuilderStreamContext.Provider>
            </BuilderThreadContext.Provider>
        </BuilderActionsContext.Provider>
    )
}
