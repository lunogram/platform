import { useContext } from "react"
import {
    BuilderActionsContext,
    BuilderThreadContext,
    BuilderStreamContext,
    type BuilderActionsContextValue,
    type BuilderThreadContextValue,
    type BuilderStreamContextValue,
} from "./BuilderContextDef"

const noop = () => {}
const noopAsync = async () => {}

/** Noop fallback used when no BuilderProvider is present (non-enterprise builds) */
const noopActions: BuilderActionsContextValue = {
    sendMessage: noopAsync,
    selectImage: noop,
    deselectImage: noop,
    setImages: noop,
    selectSection: noop,
    deselectSection: noop,
    clearSections: noop,
    setViewport: noop,
    loadTemplate: noop,
    startCompileCheck: noop,
    reportCompileResult: noop,
}

/** Stable action callbacks — never triggers re-renders */
export function useBuilderActions(): BuilderActionsContextValue {
    const context = useContext(BuilderActionsContext)
    if (!context) {
        throw new Error("useBuilderActions must be used within a BuilderProvider")
    }
    return context
}

/**
 * Like `useBuilderActions` but returns noop fallbacks instead of throwing
 * when called outside a `BuilderProvider`. Used in components that are
 * shared between enterprise and non-enterprise builds.
 */
export function useBuilderActionsOptional(): BuilderActionsContextValue {
    const context = useContext(BuilderActionsContext)
    return context ?? noopActions
}

/**
 * Thread state — messages, selected items, viewport, etc.
 * Re-renders only when messages are added/promoted or selections change.
 */
export function useBuilderThread(): BuilderThreadContextValue {
    const context = useContext(BuilderThreadContext)
    if (!context) {
        throw new Error("useBuilderThread must be used within a BuilderProvider")
    }
    return context
}

/**
 * Streaming state — segments, typing indicator, compile check.
 * Re-renders on every streaming delta — only used by the streaming
 * indicator component at the bottom of the thread.
 */
export function useBuilderStream(): BuilderStreamContextValue {
    const context = useContext(BuilderStreamContext)
    if (!context) {
        throw new Error("useBuilderStream must be used within a BuilderProvider")
    }
    return context
}
