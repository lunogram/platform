import { useContext, useEffect, useMemo, useRef } from "react"
import { createDefaultTemplateContent } from "@templatical/types"
import type { TemplateContent } from "@templatical/types"
import { toast } from "sonner"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import type { EmailDocument } from "../codeEditor/hooks/useEditorMode"
import { BlockEditor } from "./BlockEditor"

interface TemplaticalBlockEditorProps {
    /** Stored document, absent for a template that has never used this editor. */
    initialDocument?: EmailDocument
    onChange: (doc: EmailDocument) => void
}

/**
 * Adapts Templatical to the block-editor slot in the mail editor.
 *
 * The slot's contract is the opaque `EmailDocument`; Templatical's concrete
 * `TemplateContent` is confined to this file and the wrapper beneath it.
 */
export function TemplaticalBlockEditor({ initialDocument, onChange }: TemplaticalBlockEditorProps) {
    const [preferences] = useContext(PreferencesContext)

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
        <BlockEditor
            initialContent={initialContent}
            onChange={(content) => onChange(content as unknown as EmailDocument)}
            onError={(error) => toast.error(error.message)}
            theme={preferences.mode}
        />
    )
}
