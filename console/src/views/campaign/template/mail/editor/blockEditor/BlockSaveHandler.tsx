import { useContext } from "react"
import { useGetPuck } from "@puckeditor/core"
import { TemplateWorkflowContext } from "../../../contexts"
import { CampaignContext, ProjectContext, TemplateContext } from "@/mod"
import api from "@/api"
import { renderBlockToHtml } from "../handlers/renderBlockToHtml"

export default function BlockSaveHandler() {
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [template, setTemplate] = useContext(TemplateContext)
    const getPuck = useGetPuck()

    onSubmit(async () => {
        const { appState } = getPuck()

        const html = await renderBlockToHtml(appState.data, template.locale)

        const updated = await api.campaigns.templates.update(project.id, campaign.id, template.id, {
            data: {
                ...template.data,
                editor: appState.data,
                html: html,
                type: "block",
            },
        })

        setTemplate(updated)
        return true
    })

    return null
}
