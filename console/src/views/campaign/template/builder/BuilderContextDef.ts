import { createContext } from "react"
import type { Image } from "@/types"
import type { UUID } from "@/types/common"
import type {
    BuilderMessage,
    CompileCheckStatus,
    SelectedSection,
    StreamingSegment,
    ViewportMode,
} from "./types"

/** Actions context — stable callbacks that never change identity */
export interface BuilderActionsContextValue {
    sendMessage: (text: string, options?: { isAutoFix?: boolean }) => Promise<void>
    selectImage: (image: Image) => void
    deselectImage: (imageId: UUID) => void
    setImages: (images: Image[]) => void
    selectSection: (section: SelectedSection) => void
    deselectSection: (label: string) => void
    clearSections: () => void
    setViewport: (viewport: ViewportMode) => void
    loadTemplate: (source: string) => void
    /** Signal that a compile check has started (sets status to "checking") */
    startCompileCheck: () => void
    /**
     * Report the result of compiling the AI-generated template.
     * When an error is reported and the retry budget has not been exhausted,
     * a fix request is automatically sent to the AI.
     */
    reportCompileResult: (result: { success: true } | { success: false; error: string }) => void
}

/**
 * Thread context — the append-only message list and related state.
 * Only updates when a message is added/promoted (low frequency).
 */
export interface BuilderThreadContextValue {
    messages: BuilderMessage[]
    currentSource: string | null
    selectedImages: Image[]
    selectedSections: SelectedSection[]
    isAgentTyping: boolean
    versionCount: number
    viewport: ViewportMode
    conversationId: string | null
    streamError: string | null
}

/**
 * Stream context — high-frequency streaming state.
 * Isolated so that only the streaming indicator re-renders on deltas.
 */
export interface BuilderStreamContextValue {
    streamingSegments: StreamingSegment[]
    isAgentTyping: boolean
    compileCheckStatus: CompileCheckStatus
    compileCheckError: string | null
    compileFixAttempts: number
}

export const BuilderActionsContext = createContext<BuilderActionsContextValue | null>(null)
export const BuilderThreadContext = createContext<BuilderThreadContextValue | null>(null)
export const BuilderStreamContext = createContext<BuilderStreamContextValue | null>(null)
