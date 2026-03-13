import type { UUID } from "@/types/common"
import type { Image } from "@/types"

/** A finalized segment preserving the order in which text and code arrived */
export type FinalizedSegment =
    | { type: "text"; content: string }
    | {
          type: "code"
          content: string
          mode?: "write" | "range"
          description?: string
          startLine?: number
          endLine?: number
      }

/** A single message in the builder conversation */
export interface BuilderMessage {
    id: string
    role: "user" | "agent" | "system"
    /** Sub-type qualifier for the role, e.g. "compile_error" for auto-fix messages */
    kind?: string | null
    text: string
    /** Images attached as context when this message was sent */
    images?: Image[]
    /** Sections selected as context when this message was sent */
    sections?: SelectedSection[]
    /** JSX source snapshot produced by this agent turn */
    templateSource?: string
    /** Version number for agent messages that produced template changes */
    version?: number
    /** Ordered segments preserving the interleaved text/code arrival order */
    segments?: FinalizedSegment[]
    createdAt: Date
}

/** A section selected via the element selector */
export interface SelectedSection {
    /** Human-readable label derived from headings / text content */
    label: string
    /** The data-section attribute value, when present in the JSX source */
    sectionId?: string
    /** Short text excerpt from the section for fallback context */
    textContent?: string
}

/** Viewport mode for the preview panel */
export type ViewportMode = "desktop" | "mobile"

/** A segment in the streaming output — preserves the order text and code arrive */
export type StreamingSegment =
    | { type: "text"; content: string }
    | {
          type: "code"
          content: string
          mode: "write" | "range"
          description?: string
          startLine?: number
          endLine?: number
          done: boolean
      }

/** Compile-check status after AI produces new code */
export type CompileCheckStatus = "idle" | "checking" | "success" | "error"

/** State for the builder conversation */
export interface BuilderState {
    messages: BuilderMessage[]
    currentSource: string | null
    selectedImages: Image[]
    selectedSections: SelectedSection[]
    isAgentTyping: boolean
    versionCount: number
    viewport: ViewportMode
    /** Backoffice conversation ID — created lazily on first message */
    conversationId: string | null
    /** Ordered segments of text and code produced during the current stream */
    streamingSegments: StreamingSegment[]
    streamError: string | null
    /** Compile-check status after AI produces new template code */
    compileCheckStatus: CompileCheckStatus
    /** The compile error message when compileCheckStatus is "error" */
    compileCheckError: string | null
    /** Number of automatic compile-fix attempts for the current AI turn (prevents infinite loops) */
    compileFixAttempts: number
}

export const BuilderActionType = {
    SEND_MESSAGE: "SEND_MESSAGE",
    SET_AGENT_TYPING: "SET_AGENT_TYPING",
    SELECT_IMAGE: "SELECT_IMAGE",
    DESELECT_IMAGE: "DESELECT_IMAGE",
    SET_IMAGES: "SET_IMAGES",
    TOGGLE_SECTION: "TOGGLE_SECTION",
    DESELECT_SECTION: "DESELECT_SECTION",
    CLEAR_SECTIONS: "CLEAR_SECTIONS",
    SET_VIEWPORT: "SET_VIEWPORT",
    LOAD_TEMPLATE: "LOAD_TEMPLATE",
    SET_CONVERSATION_ID: "SET_CONVERSATION_ID",
    STREAM_START: "STREAM_START",
    STREAM_DELTA: "STREAM_DELTA",
    STREAM_CODE_START: "STREAM_CODE_START",
    STREAM_CODE_DELTA: "STREAM_CODE_DELTA",
    STREAM_CODE_END: "STREAM_CODE_END",
    STREAM_DONE: "STREAM_DONE",
    STREAM_ERROR: "STREAM_ERROR",
    COMPILE_CHECK_START: "COMPILE_CHECK_START",
    COMPILE_CHECK_SUCCESS: "COMPILE_CHECK_SUCCESS",
    COMPILE_CHECK_ERROR: "COMPILE_CHECK_ERROR",
} as const

export type BuilderAction =
    | {
          type: typeof BuilderActionType.SEND_MESSAGE
          text: string
          images: Image[]
          sections: SelectedSection[]
          isAutoFix?: boolean
      }
    | { type: typeof BuilderActionType.SET_AGENT_TYPING; isTyping: boolean }
    | { type: typeof BuilderActionType.SELECT_IMAGE; image: Image }
    | { type: typeof BuilderActionType.DESELECT_IMAGE; imageId: UUID }
    | { type: typeof BuilderActionType.SET_IMAGES; images: Image[] }
    | { type: typeof BuilderActionType.TOGGLE_SECTION; section: SelectedSection }
    | { type: typeof BuilderActionType.DESELECT_SECTION; label: string }
    | { type: typeof BuilderActionType.CLEAR_SECTIONS }
    | { type: typeof BuilderActionType.SET_VIEWPORT; viewport: ViewportMode }
    | { type: typeof BuilderActionType.LOAD_TEMPLATE; source: string }
    | { type: typeof BuilderActionType.SET_CONVERSATION_ID; conversationId: string }
    | { type: typeof BuilderActionType.STREAM_START; messageId: string; conversationId: string }
    | { type: typeof BuilderActionType.STREAM_DELTA; content: string }
    | {
          type: typeof BuilderActionType.STREAM_CODE_START
          mode: "write" | "range"
          description?: string
          startLine?: number
          endLine?: number
      }
    | { type: typeof BuilderActionType.STREAM_CODE_DELTA; content: string }
    | { type: typeof BuilderActionType.STREAM_CODE_END; versionId: string; versionNumber: number }
    | {
          type: typeof BuilderActionType.STREAM_DONE
          message: { id: string; role: string; content: string; created_at: string }
      }
    | { type: typeof BuilderActionType.STREAM_ERROR; error: string }
    | { type: typeof BuilderActionType.COMPILE_CHECK_START }
    | { type: typeof BuilderActionType.COMPILE_CHECK_SUCCESS }
    | { type: typeof BuilderActionType.COMPILE_CHECK_ERROR; error: string }
