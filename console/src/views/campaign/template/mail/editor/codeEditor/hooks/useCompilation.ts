import { useCallback, useEffect, useRef, useState } from "react"
import { compileEmail } from "../compileEmail"

interface UseCompilationResult {
    compiledHtml: string
    compileError: string | null
    autoPlainText: string
}

/**
 * Manages the compilation pipeline: debounced compilation of JSX source
 * into HTML/plain text, with proper abort handling for in-flight requests.
 */
export function useCompilation(
    code: string,
    previewProps: Record<string, unknown>,
): UseCompilationResult {
    const [compiledHtml, setCompiledHtml] = useState("")
    const [compileError, setCompileError] = useState<string | null>(null)
    const [autoPlainText, setAutoPlainText] = useState("")

    const compileTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
    const compileAbortRef = useRef<AbortController | null>(null)

    const runCompile = useCallback(async (source: string, props: Record<string, unknown>) => {
        if (compileAbortRef.current) {
            compileAbortRef.current.abort()
        }
        const abortController = new AbortController()
        compileAbortRef.current = abortController

        try {
            const result = await compileEmail(source, props, abortController.signal)
            if (abortController.signal.aborted) return

            setCompiledHtml(result.html)
            setCompileError(null)
            setAutoPlainText(result.plainText)
        } catch (err) {
            if (abortController.signal.aborted) return
            if (err instanceof DOMException && err.name === "AbortError") return

            console.error("[CodeEditor] Compilation error:", err)
            setCompileError(err instanceof Error ? err.message : String(err))
        }
    }, [])

    // Debounced compile on code change (also re-compile when preview props change)
    useEffect(() => {
        if (compileTimeoutRef.current) {
            clearTimeout(compileTimeoutRef.current)
        }
        compileTimeoutRef.current = setTimeout(() => {
            void runCompile(code, previewProps)
        }, 600)

        return () => {
            if (compileTimeoutRef.current) {
                clearTimeout(compileTimeoutRef.current)
            }
            if (compileAbortRef.current) {
                compileAbortRef.current.abort()
            }
        }
    }, [code, previewProps, runCompile])

    return { compiledHtml, compileError, autoPlainText }
}
