import { useRef, useEffect, useState, useCallback, memo } from "react"
import { FlaskConical } from "lucide-react"
import { useBuilderActions, useBuilderThread, useBuilderStream } from "./useBuilder"
import { ChatMessage, AgentWritingIndicator, CompileCheckIndicator } from "./ChatMessage"
import { ChatInput } from "./ChatInput"
import { EmptyState } from "./EmptyState"
import { ImageLibraryModal } from "./ImageLibraryModal"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import type { Image } from "@/types"

/**
 * Renders the append-only list of finalized messages.
 * Subscribes only to `BuilderThreadContext` — does NOT re-render
 * on streaming deltas.
 */
const MessageList = memo(function MessageList() {
    const { messages } = useBuilderThread()
    return (
        <>
            {messages.map((msg) => (
                <ChatMessage key={msg.id} message={msg} />
            ))}
        </>
    )
})

/**
 * Renders the streaming indicator (typing dots, live code, compile check).
 * Subscribes to `BuilderStreamContext` — re-renders on every streaming
 * delta, but this component is cheap and isolated from the message list.
 */
const StreamingArea = memo(function StreamingArea() {
    const {
        streamingSegments,
        isAgentTyping,
        compileCheckStatus,
        compileCheckError,
        compileFixAttempts,
    } = useBuilderStream()

    const hasStreamingContent = streamingSegments.length > 0 || isAgentTyping
    const showCompileCheck = compileCheckStatus !== "idle"

    return (
        <>
            {hasStreamingContent && (
                <AgentWritingIndicator segments={streamingSegments} isTyping={isAgentTyping} />
            )}
            {showCompileCheck && (
                <CompileCheckIndicator
                    status={compileCheckStatus}
                    error={compileCheckError}
                    attempt={compileFixAttempts}
                />
            )}
        </>
    )
})

/**
 * Auto-scroll sentinel — subscribes to both thread and stream contexts
 * to scroll on new messages or streaming activity, but renders nothing
 * visible so re-renders are essentially free.
 */
function ScrollAnchor({ anchorRef }: { anchorRef: React.RefObject<HTMLDivElement | null> }) {
    const { messages } = useBuilderThread()
    const { isAgentTyping, compileCheckStatus } = useBuilderStream()

    useEffect(() => {
        anchorRef.current?.scrollIntoView({ behavior: "smooth" })
    }, [messages.length, isAgentTyping, compileCheckStatus, anchorRef])

    return null
}

function ChatThread() {
    const bottomRef = useRef<HTMLDivElement>(null)

    return (
        <div className="relative flex-1 min-h-0">
            <ScrollArea className="h-full">
                <div className="p-4 space-y-4">
                    <MessageList />
                    <StreamingArea />
                    <ScrollAnchor anchorRef={bottomRef} />
                    <div ref={bottomRef} />
                </div>
            </ScrollArea>
            <div className="absolute bottom-2 right-3 pointer-events-none">
                <Badge className="gap-1 bg-violet-500/15 text-violet-600 dark:text-violet-400 border-violet-500/25 hover:bg-violet-500/15 text-[10px] font-medium px-2 py-0.5 pointer-events-auto">
                    <FlaskConical className="h-3 w-3 shrink-0" />
                    Alpha — outputs may need manual adjustments
                </Badge>
            </div>
        </div>
    )
}

function ConversationPanel({ onAddImages }: { onAddImages: () => void }) {
    return (
        <div className="flex flex-col h-full">
            <ChatThread />
            <ChatInput onAddImages={onAddImages} />
        </div>
    )
}

interface BuilderPanelProps {
    /** Current JSX source — synced from the parent Editor */
    currentSource: string | null
    /** Callback when the builder produces new JSX source */
    onSourceChange: (source: string) => void
}

/**
 * Builder panel — embeddable within the Editor view.
 * Renders either the empty state (no messages yet) or the
 * chat conversation panel. The parent Editor owns the
 * BuilderProvider wrapper and passes source state down.
 */
export function BuilderPanel({ currentSource, onSourceChange }: BuilderPanelProps) {
    const { setImages, loadTemplate } = useBuilderActions()
    const thread = useBuilderThread()
    const [imageModalOpen, setImageModalOpen] = useState(false)

    // Sync incoming source into builder state on mount
    const loadedRef = useRef(false)
    useEffect(() => {
        if (!loadedRef.current && currentSource) {
            loadTemplate(currentSource)
            loadedRef.current = true
        }
    }, [currentSource, loadTemplate])

    // When the builder produces new source, notify the parent
    const prevSourceRef = useRef(thread.currentSource)
    useEffect(() => {
        if (thread.currentSource && thread.currentSource !== prevSourceRef.current) {
            prevSourceRef.current = thread.currentSource
            onSourceChange(thread.currentSource)
        }
    }, [thread.currentSource, onSourceChange])

    const hasMessages = thread.messages.length > 0

    const openImageModal = useCallback(() => {
        setImageModalOpen(true)
    }, [])

    const handleImageSelect = useCallback(
        (images: Image[]) => {
            setImages(images)
        },
        [setImages],
    )

    return (
        <>
            {hasMessages ? (
                <ConversationPanel onAddImages={openImageModal} />
            ) : (
                <EmptyState onAddImages={openImageModal} />
            )}
            <ImageLibraryModal
                open={imageModalOpen}
                onClose={setImageModalOpen}
                onSelect={handleImageSelect}
                selectedIds={thread.selectedImages.map((img) => img.id)}
            />
        </>
    )
}
