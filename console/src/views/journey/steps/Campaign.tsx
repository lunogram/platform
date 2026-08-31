import { useCallback } from "react"
import api from "../../../api"
import type { Campaign, CampaignVariable, JourneyStepType } from "../../../types"
import type { VariableGroup } from "../JourneyVariableContext"
import { Combobox } from "@/components/ui/combobox"
import { Label } from "@/components/ui/label"
import { ActionStepIcon } from "../../../components/icons"
import { useResolver } from "../../../hooks"
import { useTranslation } from "react-i18next"
import { ChannelIcon } from "../../campaign/ChannelTag"
import { CreateCampaign } from "../../campaign/CreateCampaign"
import Preview from "@/components/preview"
import type { UUID } from "@/types/common"
import { NIL } from "uuid"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { TemplateInput } from "@/components/ui/template-input"
import { useJourneyVariableContext } from "../JourneyVariableContext"
import { isEnterprise } from "@/config/enterprise"

interface CampaignConfig {
    campaign_id: UUID
    data?: Record<string, string>
    variant?: string
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
    Edit({ project, onChange, value, onSaveDraft, nodeId }) {
        const { t } = useTranslation()
        const projectId = project.id
        const { getVariablesForNode } = useJourneyVariableContext()

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
                    search: query || undefined,
                    limit: 50,
                })
                return result.results.map((c) => ({ ...c, path: c.id }))
            },
            [projectId],
        )

        const variables = campaign?.variables ?? []
        const campaignVariants = campaign?.variants ?? []
        const journeyVariables = nodeId ? getVariablesForNode(nodeId) : []

        const handleVariableChange = (name: string, newValue: string) => {
            onChange({
                ...value,
                data: { ...value.data, [name]: newValue },
            })
        }

        function VariableRow({
            variable,
            rowValue,
            onRowChange,
            vars,
        }: {
            variable: CampaignVariable
            rowValue: string
            onRowChange: (value: string) => void
            vars: VariableGroup[]
        }) {
            const hasDefault = variable.default !== undefined && variable.default !== ""

            return (
                <div className="space-y-1">
                    <Label className="text-sm font-medium">
                        {variable.name}
                        {!hasDefault && <span className="text-destructive"> *</span>}
                    </Label>
                    <TemplateInput
                        value={rowValue}
                        onChange={onRowChange}
                        variables={vars}
                        placeholder={variable.default ?? variable.name}
                    />
                    {hasDefault && (
                        <p className="text-xs text-muted-foreground">
                            &darr; Default: {variable.default}
                        </p>
                    )}
                </div>
            )
        }

        return (
            <div className="space-y-3">
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
                        onValueChange={(id) =>
                            onChange({
                                ...value,
                                campaign_id: (id || NIL) as UUID,
                                data: {},
                                variant: undefined,
                            })
                        }
                        placeholder={t("campaign.singular")}
                        renderOption={(option) => option.name}
                    />
                </div>
                <div className="flex items-center gap-2 pt-1">
                    <div className="h-px flex-1 bg-border" />
                    <span className="text-xs text-muted-foreground">{t("or")}</span>
                    <div className="h-px flex-1 bg-border" />
                </div>
                <CreateCampaign
                    onBeforeCreate={onSaveDraft}
                    trigger={
                        <Button variant="outline" size="sm" className="w-full">
                            <PlusIcon className="h-4 w-4" />
                            {t("campaign.create.action")}
                        </Button>
                    }
                />

                {isEnterprise && campaign && campaignVariants.length > 0 && (
                    <div className="space-y-1.5 border-t pt-3">
                        <Label className="text-sm font-medium">
                            {t("campaign.variants.title", "Variant")}
                        </Label>
                        <p className="text-xs text-muted-foreground">
                            {t(
                                "journey.campaign.variant_description",
                                "Which design this step sends. Leave empty to let the campaign decide per recipient.",
                            )}
                        </p>
                        <TemplateInput
                            value={value.variant ?? ""}
                            onChange={(newValue) => onChange({ ...value, variant: newValue })}
                            variables={journeyVariables}
                            placeholder={campaignVariants.map((v) => v.key).join(", ")}
                        />
                    </div>
                )}

                {campaign && variables.length > 0 && (
                    <div className="space-y-3 border-t pt-3">
                        <div>
                            <Label className="text-sm font-medium">
                                {t("campaign.variables.mapping_title", "Variable Mapping")}
                            </Label>
                            <p className="text-xs text-muted-foreground mt-0.5">
                                {t(
                                    "campaign.variables.mapping_description",
                                    "Map journey data to this campaign's variables.",
                                )}
                            </p>
                        </div>
                        {variables.map((v) => (
                            <VariableRow
                                key={v.name}
                                variable={v}
                                rowValue={value.data?.[v.name] ?? ""}
                                onRowChange={(newValue) => handleVariableChange(v.name, newValue)}
                                vars={journeyVariables}
                            />
                        ))}
                    </div>
                )}

                {campaign && variables.length === 0 && (
                    <p className="text-xs text-muted-foreground italic border-t pt-3">
                        {t(
                            "campaign.variables.none_defined",
                            "This campaign has no variables defined.",
                        )}
                    </p>
                )}
            </div>
        )
    },
    validate: ({ campaign_id }) => {
        return !!campaign_id && campaign_id !== NIL
    },
}
