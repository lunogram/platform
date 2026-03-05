import { useContext } from "react"
import { TemplateWorkflowContext } from "../../../contexts"
import { CampaignContext, ProjectContext, TemplateContext } from "@/mod"
import api from "@/api"
import type CodeStore from "../CodeEditorPlugins/CodeStore"
import type CodeEditorEventListener from "../CodeEditorPlugins/CodeEditorEventListener"

export default function HtmlSaveHandler(props: {
    eventListener: typeof CodeEditorEventListener
    codeStore: typeof CodeStore
}) {
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [template, setTemplate] = useContext(TemplateContext)

    onSubmit(async () => {
        const updated = await api.campaigns.templates.update(project.id, campaign.id, template.id, {
            data: {
                ...template.data,
                html: props.codeStore.current,
                type: "code",
            },
        })

        setTemplate(updated)
        return true
    })

    return null
}
