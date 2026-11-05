import { useCallback, useContext } from "react"
import { TemplateContext } from "@/contexts"
import type { EmailTemplateData } from "@/types"
import { knownTemplateData } from "@/validation/campaign/template/data"
import type { EmailDocument, EditorMode } from "../codeEditor/hooks/useEditorMode"
import { saveWith, type SaveStep } from "../../../contexts"
import { useUpdateTemplate } from "../../../../queries"

interface UseEmailTemplateSaveOptions {
    editorMode: EditorMode
    code: string
    blocksData: EmailDocument | null
    useCustomPlainText: boolean
    customPlainText: string
}

/** @returns the save of the email editor step: its code, blocks and plain text. */
export function useEmailTemplateSave({
    editorMode,
    code,
    blocksData,
    useCustomPlainText,
    customPlainText,
}: UseEmailTemplateSaveOptions): SaveStep {
    const [template] = useContext(TemplateContext)
    const { mutateAsync: updateTemplate } = useUpdateTemplate()

    return useCallback(() => {
        const changes: Partial<EmailTemplateData> = {
            type: "react-email",
            editorMode,
            code: { source: code },
            blocks: blocksData ?? undefined,
            // Only the custom override is stored. The backend renders the
            // generated plain text itself at send time.
            plaintext: useCustomPlainText && customPlainText ? { custom: customPlainText } : {},
        }

        // sender_identity_id is left out, which keeps the sender the Content step set.
        return saveWith(() =>
            updateTemplate({
                templateId: template.id,
                params: { data: knownTemplateData("email", { ...template.data, ...changes }) },
            }),
        )
    }, [
        template,
        updateTemplate,
        editorMode,
        code,
        blocksData,
        useCustomPlainText,
        customPlainText,
    ])
}
