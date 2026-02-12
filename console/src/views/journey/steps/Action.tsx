import { useCallback } from 'react'
import api from '../../../api'
import type { JourneyStepType } from '../../../types'
import { ActionStepIcon } from '../../../components/icons'
import { CampaignSelect } from '@/components/campaign/select'
import { useResolver } from '../../../hooks'
import { useTranslation } from 'react-i18next'
import { ChannelIcon } from '../../campaign/ChannelTag'
import Preview from '../../../ui/Preview'
import type { UUID } from '@/types/common'
import { NIL } from 'uuid'

interface ActionConfig {
    campaign_id: UUID
}

export const actionStep: JourneyStepType<ActionConfig> = {
    name: 'send',
    icon: <ActionStepIcon />,
    category: 'action',
    description: 'send_desc',
    Describe({
        project: { id: projectId },
        value: {
            campaign_id,
        },
    }) {
        const { t } = useTranslation()
        const [campaign] = useResolver(useCallback(async () => {
            if (campaign_id) {
                return await api.campaigns.get(projectId, campaign_id)
            }
            return null
        }, [projectId, campaign_id]))
        const template = campaign?.templates?.[0]

        return (
            <>
                <div className="journey-step-body-name">
                    <div className="journey-step-action-type">
                        {campaign && <ChannelIcon channel={campaign.channel} />}
                    </div>
                    {campaign?.name ?? <>&#8211;</>}
                </div>
                <div className="journey-step-action-preview">
                    {campaign && template
                        ? <Preview template={template} size="small" />
                        : (
                            <div className="journey-step-action-preview-placeholder">{t('journey_campaign_create_preview')}</div>
                        )}
                </div>
            </>
        )
    },
    newData: async () => ({
        campaign_id: NIL as UUID,
    }),
    Edit({
        project: { id: projectId },
        onChange,
        value,
    }) {
        const { t } = useTranslation()
        return (
            <div className="grid gap-2">
                <label className="text-sm font-medium">{t('campaign.singular')}</label>
                <p className="text-sm text-muted-foreground">{t('send_campaign_desc')}</p>
                <CampaignSelect
                    projectId={projectId}
                    value={value.campaign_id}
                    onChange={campaign_id => onChange({ ...value, campaign_id: campaign_id ?? NIL as UUID })}
                    required
                />
            </div>
        )
    },
    validate: ({ campaign_id }) => {
        return !!campaign_id && campaign_id !== NIL
    },
    hasDataKey: true,
}
