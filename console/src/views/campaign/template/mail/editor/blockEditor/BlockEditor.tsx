import { useEffect, useRef } from "react"
import { init } from "@templatical/editor"
import type { TemplaticalEditor } from "@templatical/editor"
import type { TemplateContent } from "@templatical/types"
import "@templatical/editor/style.css"

interface BlockEditorProps {
    /**
     * Document to open. Read once, on mount — the editor owns its content
     * afterwards and reports changes through `onChange`. Passing a new object
     * on every keystroke would otherwise remount the editor.
     */
    initialContent: TemplateContent
    onChange: (content: TemplateContent) => void
    onError?: (error: Error) => void
    theme: "light" | "dark"
}

/**
 * React wrapper around Templatical, which mounts imperatively into a DOM node
 * rather than rendering as a component.
 *
 * The editor renders into a Shadow DOM by default, so the console's Tailwind
 * cannot cascade into it and its own classes cannot collide with ours.
 *
 * Layout caveat from the library: no ancestor of the container may set
 * `transform`, `filter`, `perspective` or `will-change`. Each establishes a
 * containing block for `position: fixed`, which detaches the editor's floating
 * UI (colour pickers, text toolbars) and drag ghost from their anchors.
 */
export function BlockEditor({ initialContent, onChange, onError, theme }: BlockEditorProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const editorRef = useRef<TemplaticalEditor | null>(null)

    // Held in refs so a changed callback identity never remounts the editor.
    const onChangeRef = useRef(onChange)
    onChangeRef.current = onChange
    const onErrorRef = useRef(onError)
    onErrorRef.current = onError
    const initialContentRef = useRef(initialContent)

    useEffect(() => {
        const container = containerRef.current
        if (!container) return

        // `init` is async, and StrictMode invokes this effect twice in
        // development. `disposed` covers the window where cleanup runs before
        // the promise settles — without it the first editor would be orphaned,
        // still mounted and still emitting changes.
        let disposed = false
        let instance: TemplaticalEditor | null = null

        void init({
            container,
            content: initialContentRef.current,
            // Liquid matches the platform's own template syntax, so merge tags
            // authored here resolve in the send pipeline unchanged.
            mergeTags: { syntax: "liquid" },
            onChange: (content) => onChangeRef.current(content),
            onError: (error) => onErrorRef.current?.(error),
        })
            .then((editor) => {
                if (disposed) {
                    editor.unmount()
                    return
                }
                instance = editor
                editorRef.current = editor
            })
            .catch((error: unknown) => {
                onErrorRef.current?.(error instanceof Error ? error : new Error(String(error)))
            })

        return () => {
            disposed = true
            instance?.unmount()
            editorRef.current = null
        }
    }, [])

    useEffect(() => {
        editorRef.current?.setTheme(theme)
    }, [theme])

    return <div ref={containerRef} className="h-full w-full" />
}
