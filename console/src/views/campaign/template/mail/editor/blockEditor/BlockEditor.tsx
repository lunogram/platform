import { useEffect, useRef } from "react"
import { init } from "@templatical/editor"
import type { TemplaticalEditor } from "@templatical/editor"
import type { TemplateContent } from "@templatical/types"
import type { TemplaticalMergeTag } from "./mergeTags"
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
    /** Offered by the tag picker and typing autocomplete. Read once, on mount. */
    mergeTags: TemplaticalMergeTag[]
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
export function BlockEditor({
    initialContent,
    onChange,
    onError,
    theme,
    mergeTags,
}: BlockEditorProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const editorRef = useRef<TemplaticalEditor | null>(null)

    // Held in refs so a changed callback identity never remounts the editor.
    const onChangeRef = useRef(onChange)
    onChangeRef.current = onChange
    const onErrorRef = useRef(onError)
    onErrorRef.current = onError
    const initialContentRef = useRef(initialContent)
    const themeRef = useRef(theme)
    themeRef.current = theme
    const mergeTagsRef = useRef(mergeTags)

    useEffect(() => {
        const host = containerRef.current
        if (!host) return

        // Every mount gets its own node rather than sharing the ref'd element.
        // `init` is async and StrictMode invokes this effect twice, so two
        // editors can be initialising into the same parent at once; whichever
        // settles second would have its DOM torn down by the other's unmount,
        // leaving an empty shadow root and a blank editor.
        const mountPoint = document.createElement("div")
        mountPoint.className = "h-full w-full"
        host.appendChild(mountPoint)

        // Covers the window where cleanup runs before the promise settles —
        // without it the editor would be orphaned, still mounted and still
        // emitting changes.
        let disposed = false
        let instance: TemplaticalEditor | null = null

        void init({
            container: mountPoint,
            content: initialContentRef.current,
            // Set at init as well as through setTheme below, so the editor's
            // first paint already matches the console instead of flashing the
            // library default.
            uiTheme: themeRef.current,
            // Liquid matches the platform's own template syntax, so merge tags
            // authored here resolve in the send pipeline unchanged.
            mergeTags: { syntax: "liquid", tags: mergeTagsRef.current },
            onChange: (content) => onChangeRef.current(content),
            onError: (error) => onErrorRef.current?.(error),
        })
            .then((editor) => {
                if (disposed) {
                    editor.unmount()
                    mountPoint.remove()
                    return
                }
                instance = editor
                editorRef.current = editor
                hideDarkModePreview(mountPoint)
            })
            .catch((error: unknown) => {
                onErrorRef.current?.(error instanceof Error ? error : new Error(String(error)))
            })

        return () => {
            disposed = true
            instance?.unmount()
            mountPoint.remove()
            // Only clear the shared ref if it still points at this instance —
            // a concurrent mount may already have replaced it.
            if (editorRef.current === instance) {
                editorRef.current = null
            }
        }
    }, [])

    useEffect(() => {
        editorRef.current?.setTheme(theme)
    }, [theme])

    return <div ref={containerRef} className="h-full w-full" />
}

/**
 * Hide the editor's built-in dark-mode preview control.
 *
 * It toggles a preview of how the *email* renders in a dark-mode mail client,
 * which is a separate concern from the console's own appearance and is not
 * something this integration supports yet — the backend renders one HTML
 * variant, so what the toggle previews would not match what is sent.
 *
 * The editor has no config option for it, and its UI lives behind a shadow
 * boundary that host stylesheets cannot cross, so the rule is injected into
 * the shadow root itself.
 */
function hideDarkModePreview(mountPoint: HTMLElement) {
    const host = mountPoint.shadowRoot
        ? mountPoint
        : mountPoint.querySelector<HTMLElement>("*:not(style)")
    const root = host?.shadowRoot ?? findShadowRoot(mountPoint)
    if (!root) return

    const style = document.createElement("style")
    style.textContent = ".tpl-dark-mode-toggle { display: none !important; }"
    root.appendChild(style)
}

function findShadowRoot(node: HTMLElement): ShadowRoot | null {
    if (node.shadowRoot) return node.shadowRoot
    for (const child of node.querySelectorAll("*")) {
        const shadow = (child as HTMLElement).shadowRoot
        if (shadow) return shadow
    }
    return null
}
