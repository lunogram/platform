import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { createDefaultTemplateContent } from "@templatical/types"
import type { TemplateContent } from "@templatical/types"
import { toast } from "sonner"
import type { EmailDocument } from "../codeEditor/hooks/useEditorMode"
import { BlockEditor } from "./BlockEditor"
import { MediaManager } from "@/components/media-manager"
import type { Image } from "@/types"
import { useConsoleTheme } from "./useConsoleTheme"
import { toMergeTags } from "./mergeTags"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

interface TemplaticalBlockEditorProps {
    /** Stored document, absent for a template that has never used this editor. */
    initialDocument?: EmailDocument
    onChange: (doc: EmailDocument) => void
    /** Campaign variables, offered as merge tags in the editor. */
    variableGroups: VariableGroup[]
}

/**
 * Adapts Templatical to the block-editor slot in the mail editor.
 *
 * The slot's contract is the opaque `EmailDocument`; Templatical's concrete
 * `TemplateContent` is confined to this file and the wrapper beneath it.
 */
export function TemplaticalBlockEditor({
    initialDocument,
    onChange,
    variableGroups,
}: TemplaticalBlockEditorProps) {
    const theme = useConsoleTheme()
    // Read once on mount, matching how the editor consumes them.
    const mergeTags = useMemo(() => toMergeTags(variableGroups), [variableGroups])

    // The editor asks for media imperatively and waits on a promise, while the
    // media manager is a React modal. Park the resolver until the user either
    // picks an image or dismisses the dialog, so the editor is never left
    // waiting on a promise that cannot settle.
    const [mediaOpen, setMediaOpen] = useState(false)
    const resolveMediaRef = useRef<((result: { url: string; alt?: string } | null) => void) | null>(
        null,
    )

    const requestMedia = useCallback(
        () =>
            new Promise<{ url: string; alt?: string } | null>((resolve) => {
                resolveMediaRef.current = resolve
                setMediaOpen(true)
            }),
        [],
    )

    const settleMedia = useCallback((result: { url: string; alt?: string } | null) => {
        resolveMediaRef.current?.(result)
        resolveMediaRef.current = null
    }, [])

    // Resolve any pending request on unmount for the same reason.
    useEffect(() => () => settleMedia(null), [settleMedia])

    // A template switched over from the code editor has no document yet, so
    // start from an empty one rather than mounting the editor with nothing.
    const seededRef = useRef(initialDocument === undefined)
    const initialContent = useMemo<TemplateContent>(
        () => (initialDocument as TemplateContent | undefined) ?? createDefaultTemplateContent(),
        // Read once: the editor owns the document after mount.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [],
    )

    const onChangeRef = useRef(onChange)
    onChangeRef.current = onChange

    // Report a freshly seeded document upward immediately. Templatical only
    // emits onChange once the user edits something, so without this a template
    // saved straight after switching would store no document at all — and the
    // backend, told by data.type to compile the document, would have nothing to
    // compile and the campaign would fail to send.
    useEffect(() => {
        if (seededRef.current) {
            onChangeRef.current(initialContent as unknown as EmailDocument)
        }
    }, [initialContent])

    return (
        <>
            <BlockEditor
                initialContent={initialContent}
                onChange={(content) => onChange(content as unknown as EmailDocument)}
                onError={(error) => toast.error(error.message)}
                theme={theme}
                mergeTags={mergeTags}
                onRequestMedia={requestMedia}
            />
            <MediaManager
                open={mediaOpen}
                onOpenChange={(open) => {
                    if (!open) settleMedia(null)
                    setMediaOpen(open)
                }}
                onSelect={(image: Image) => {
                    settleMedia({ url: image.url, alt: image.name })
                    setMediaOpen(false)
                }}
            />
        </>
    )
}
