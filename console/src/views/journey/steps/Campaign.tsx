import { useCallback } from "react"
import api from "../../../api"
import type { Campaign, JourneyStepType } from "../../../types"
import { Combobox } from "@/components/ui/combobox"
import { Label } from "@/components/ui/label"
import { ActionStepIcon } from "../../../components/icons"
import { CreateCampaign } from "@/views/campaign/CreateCampaign"
import { useResolver } from "../../../hooks"
import { useTranslation } from "react-i18next"
import { ChannelIcon } from "../../campaign/ChannelTag"
import Preview from "@/components/preview"
import type { UUID } from "@/types/common"
import { NIL } from "uuid"

interface CampaignConfig {
    campaign_id: UUID
}

type CampaignOption = Campaign & { path: string }

export const campaignStep: JourneyStepType<CampaignConfig> = {
    name: "send",
    icon: <ActionStepIcon />,
    category: "action",
    description: "send_desc",
    Describe({ project: { id: projectId }, value: { campaign_id } }) {
        const { t } = useTranslation()
        const [campaign] = useResolver(
            useCallback(async () => {
                if (campaign_id) {
                    return await api.campaigns.get(projectId, campaign_id)
                }
                return null
            }, [projectId, campaign_id]),
        )
        const template = campaign?.templates?.[0]

        return (
            <div className="space-y-2.5 max-w-[300px]">
                <div className="flex items-center gap-2.5 font-bold">
                    <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-md bg-muted">
                        {campaign && <ChannelIcon channel={campaign.channel} />}
                    </div>
                    <span className="truncate">{campaign?.name ?? <>&#8211;</>}</span>
                </div>
                <div className="flex h-[200px] w-[250px] items-center justify-center rounded-md bg-muted">
                    {campaign && template ? (
                        <Preview template={template} size="small" />
                    ) : (
                        <span className="text-sm font-medium text-muted-foreground">
                            {t("journey_campaign_create_preview")}
                        </span>
                    )}
                </div>
            </div>
        )
    },
    newData: async () => ({
        campaign_id: NIL as UUID,
    }),
    Edit({ project: { id: projectId }, onChange, value }) {
        const { t } = useTranslation()
        const [campaign] = useResolver(
            useCallback(async () => {
                if (value.campaign_id && value.campaign_id !== NIL) {
                    return await api.campaigns.get(projectId, value.campaign_id)
                }
                return null
            }, [projectId, value.campaign_id]),
        )

        const handleSearch = useCallback(
            async (query: string): Promise<CampaignOption[]> => {
                const result = await api.campaigns.search(projectId, {
                    search: query,
                    limit: 50,
                    filter: { type: "trigger" },
                })
                return result.results.map((c) => ({ ...c, path: c.id }))
            },
            [projectId],
        )

        return (
            <div className="space-y-1.5">
                <Label className="text-sm font-medium">
                    {t("campaign.singular")}
                    <span className="text-destructive"> *</span>
                </Label>
                <p className="text-sm text-muted-foreground">{t("send_campaign_desc")}</p>
                <Combobox<CampaignOption>
                    onSearch={handleSearch}
                    value={value.campaign_id === NIL ? "" : value.campaign_id}
                    displayValue={campaign?.name}
                    onValueChange={(id) => onChange({ ...value, campaign_id: (id || NIL) as UUID })}
                    placeholder={t("campaign.singular")}
                    renderOption={(option) => option.name}
                />
                <CreateCampaign />
            </div>
        )
    },
    validate: ({ campaign_id }) => {
        return !!campaign_id && campaign_id !== NIL
    },
    hasDataKey: true,
}
