import { useContext, useEffect } from "react"
import { TemplateWorkflowContext } from "../../../contexts"
import { TemplateContext } from "@/contexts"
import type { EmailDocument } from "../codeEditor/hooks/useEditorMode"
import type { EditorMode } from "../codeEditor/hooks/useEditorMode"
import { useUpdateTemplate } from "../../../../queries"

interface UseTemplatePersistenceOptions {
    editorMode: EditorMode
    code: string
    blocksData: EmailDocument | null
    useCustomPlainText: boolean
    customPlainText: string
}

/**
 * Subscribes to the template workflow's onSubmit event and persists
 * the current editor state (code, blocks, plain text) to the API.
 */
export function useTemplatePersistence({
    editorMode,
    code,
    blocksData,
    useCustomPlainText,
    customPlainText,
}: UseTemplatePersistenceOptions): void {
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [template] = useContext(TemplateContext)
    const { mutateAsync: updateTemplate } = useUpdateTemplate()

    useEffect(() => {
        const unsubscribe = onSubmit(async () => {
            await updateTemplate({
                templateId: template.id,
                params: {
                    data: {
                        ...template.data,
                        type: "react-email",
                        editorMode,
                        code: {
                            source: code,
                        },
                        ...(blocksData ? { blocks: blocksData } : { blocks: undefined }),
                        // Only the custom override is stored. The backend renders the
                        // generated plain text itself at send time.
                        plaintext:
                            useCustomPlainText && customPlainText
                                ? { custom: customPlainText }
                                : {},
                    },
                },
            })
            return true
        })
        return unsubscribe
    }, [
        onSubmit,
        updateTemplate,
        template,
        editorMode,
        code,
        blocksData,
        useCustomPlainText,
        customPlainText,
    ])
}
