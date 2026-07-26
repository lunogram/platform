import { useContext, useEffect } from "react"
import { TemplateWorkflowContext } from "../../../contexts"
import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import type { EmailDocument } from "../codeEditor/hooks/useEditorMode"
import type { EditorMode } from "../codeEditor/hooks/useEditorMode"
import { BLOCKS_MODE } from "../codeEditor/hooks/useEditorMode"
import api from "@/api"

interface UseTemplatePersistenceOptions {
    editorMode: EditorMode
    code: string
    blocksData: EmailDocument | null
    autoPlainText: string
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
    autoPlainText,
    useCustomPlainText,
    customPlainText,
}: UseTemplatePersistenceOptions): void {
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [template, setTemplate] = useContext(TemplateContext)

    useEffect(() => {
        const unsubscribe = onSubmit(async () => {
            const updated = await api.campaigns.templates.update(
                project.id,
                campaign.id,
                template.id,
                {
                    data: {
                        ...template.data,
                        // Selects which document the backend compiles. It must
                        // follow the mode: left pinned to react-email, a
                        // template switched to the visual editor would save its
                        // document but keep compiling stale JSX.
                        type: editorMode === BLOCKS_MODE ? "templatical" : "react-email",
                        editorMode,
                        code: {
                            source: code,
                        },
                        // Both representations are kept once they exist, so
                        // switching modes never destroys the other one.
                        ...(blocksData ? { blocks: blocksData } : {}),
                        plaintext: {
                            generated: autoPlainText,
                            ...(useCustomPlainText && customPlainText
                                ? { custom: customPlainText }
                                : {}),
                        },
                    },
                },
            )

            setTemplate(updated)
            return true
        })
        return unsubscribe
    }, [
        onSubmit,
        project.id,
        campaign.id,
        template,
        editorMode,
        code,
        blocksData,
        autoPlainText,
        useCustomPlainText,
        customPlainText,
        setTemplate,
    ])
}
