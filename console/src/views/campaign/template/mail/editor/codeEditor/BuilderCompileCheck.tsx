import { useEffect, useRef } from "react"
import {
    useBuilderActions,
    useBuilderStream,
    useBuilderThread,
} from "@lunogram-enterprise/ai-builder"
import { compileEmail } from "./compileEmail"

/**
 * Headless component that subscribes to builder stream/thread contexts
 * and runs the compile-check feedback loop when the AI finishes streaming.
 * Rendered as a child so CodeEditorInner itself doesn't re-render on
 * builder state changes.
 */
export function BuilderCompileCheck({
    previewPropsRef,
}: {
    previewPropsRef: React.RefObject<Record<string, unknown>>
}) {
    const { reportCompileResult, startCompileCheck } = useBuilderActions()
    const { currentSource: builderCurrentSource } = useBuilderThread()
    const { isAgentTyping: builderIsAgentTyping } = useBuilderStream()

    const prevAgentTypingRef = useRef(builderIsAgentTyping)
    const compileCheckAbortRef = useRef<AbortController | null>(null)

    useEffect(() => {
        const wasTyping = prevAgentTypingRef.current
        prevAgentTypingRef.current = builderIsAgentTyping

        if (wasTyping && !builderIsAgentTyping && builderCurrentSource) {
            const source = builderCurrentSource

            compileCheckAbortRef.current?.abort()
            const controller = new AbortController()
            compileCheckAbortRef.current = controller

            startCompileCheck()

            void (async () => {
                try {
                    await compileEmail(source, previewPropsRef.current, controller.signal)
                    if (!controller.signal.aborted) {
                        reportCompileResult({ success: true })
                    }
                } catch (err) {
                    if (controller.signal.aborted) return
                    if (err instanceof DOMException && err.name === "AbortError") return

                    const errorMessage = err instanceof Error ? err.message : String(err)
                    console.error("[COMPILE_CHECK] compile error:", errorMessage)
                    reportCompileResult({ success: false, error: errorMessage })
                }
            })()
        }
    }, [
        builderIsAgentTyping,
        builderCurrentSource,
        previewPropsRef,
        reportCompileResult,
        startCompileCheck,
    ])

    useEffect(() => {
        return () => {
            compileCheckAbortRef.current?.abort()
        }
    }, [])

    return null
}
