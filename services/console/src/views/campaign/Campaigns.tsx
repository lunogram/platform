import { useCallback, useContext } from 'react'
import { useNavigate } from 'react-router'
import api from '../../api'

import { LinkButton } from '../../ui/Button'
import { ArchiveIcon, DuplicateIcon, EditIcon } from '../../components/icons'
import Menu, { MenuItem } from '../../ui/Menu'
import PageContent from '../../ui/PageContent'
import { SearchTable, useSearchTableQueryState } from '../../ui/SearchTable'
import type { TagVariant } from '../../ui/Tag';
import Tag from '../../ui/Tag'
import { formatDate, snakeToTitle } from '../../utils'
import { ChannelIcon } from './ChannelTag'
import { Alert } from '../../ui'
import { ProjectContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'
import { Translation, useTranslation } from 'react-i18next'

import { CreateCampaign } from './CreateCampaign'

import type { Campaign, CampaignDelivery, CampaignState } from '@/types'
import type { UUID } from '@/types/common'
import { TypeSelect } from '@/ui/TypeSelect'

export const CampaignTag = ({ state, progress, send_at }: Pick<Campaign, 'state' | 'progress' | 'send_at'>) => {
    const variant: Record<CampaignState, TagVariant> = {
        draft: 'plain',
        aborted: 'error',
        aborting: 'error',
        loading: 'info',
        scheduled: 'info',
        running: 'info',
        finished: 'success',
    }

    const complete = progress?.complete ?? 0
    const total = progress?.total ?? 0
    const percent = total > 0 ? complete / total : 0
    const percentStr = percent.toLocaleString(undefined, { style: 'percent', minimumFractionDigits: 0 })

    const label = state === 'aborting' && send_at ? 'rescheduling' : state

    return <Tag variant={variant[state]}>
        <Translation>{(t) => t(label)}</Translation>
        {progress && ` (${percentStr})`}
    </Tag>
}

export const DeliveryRatio = ({ delivery }: { delivery: CampaignDelivery }) => {
    const sent = (delivery?.sent ?? 0)
    const total = (delivery?.total ?? 0)
    const ratio = sent > 0 ? sent / total : 0
    const sentStr = sent.toLocaleString()
    const ratioStr = ratio.toLocaleString(undefined, { style: 'percent', minimumFractionDigits: 0 })
    return `${sentStr} (${ratioStr})`
}

export const OpenRate = ({ delivery }: { delivery: CampaignDelivery }) => {
    const opens = (delivery?.opens ?? 0)
    const sent = (delivery?.sent ?? 0)
    const ratio = sent > 0 ? opens / sent : 0
    const opensStr = opens.toLocaleString()
    const ratioStr = ratio.toLocaleString(undefined, { style: 'percent', minimumFractionDigits: 0 })
    return `${opensStr} (${ratioStr})`
}

export const ClickRate = ({ delivery }: { delivery: CampaignDelivery }) => {
    const clicks = (delivery?.clicks ?? 0)
    const sent = (delivery?.sent ?? 0)
    const ratio = sent > 0 ? clicks / sent : 0
    const clicksStr = clicks.toLocaleString()
    const ratioStr = ratio.toLocaleString(undefined, { style: 'percent', minimumFractionDigits: 0 })
    return `${clicksStr} (${ratioStr})`
}

const campaignTypes = [
    { key: 'all', label: 'All' },
    { key: 'blast', label: 'Blast' },
    { key: 'trigger', label: 'Journey' },
]

interface CampaignsProps {
    create?: boolean
}

export default function Campaigns({ create = false }: CampaignsProps) {
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()

    const options = {
        filter: {
            type: '',
        },
    }

    const state = useSearchTableQueryState(
        useCallback(async params => await api.campaigns.search(project.id, params), [project.id]),
        options,
    )

    const handleEditCampaign = async (id: UUID) => {
        await navigate(`/projects/${project.id}/campaigns/${id.toString()}`)
    }

    const handleDuplicateCampaign = async (id: UUID) => {
        const campaign = await api.campaigns.duplicate(project.id, id)
        await navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}`)
    }

    const handleArchiveCampaign = async (id: UUID) => {
        await api.campaigns.delete(project.id, id)
        await state.reload()
    }

    return (
        <>
            <PageContent title={t('campaign.plural')} actions={
                <CreateCampaign open={create} />
            } banner={project.has_provider === false && (
                <Alert
                    variant="plain"
                    title={t('setup')}
                    actions={
                        <LinkButton to={`/projects/${project.id}/settings/integrations`}>{t('setup_integration')}</LinkButton>
                    }
                >{t('setup_integration_description')}</Alert>
            )}>
                <SearchTable
                    {...state}
                    emptyMessage={t('no_campaigns_found')}
                    columns={[
                        {
                            key: 'name',
                            title: t('name'),
                            sortable: true,
                            minWidth: '225px',
                            cell: ({ item: { name, channel } }) => (
                                <div className="multi-cell">
                                    <div className="placeholder">
                                        <ChannelIcon channel={channel} />
                                    </div>
                                    <div className="text">
                                        <div className="title">{name}</div>
                                        <div className="subtitle">
                                            {snakeToTitle(channel)}</div>
                                    </div>
                                </div>
                            ),
                        },
                        {
                            key: 'state',
                            title: t('state'),
                            sortable: true,
                            cell: ({ item: { state, send_at } }) => CampaignTag({ state, send_at }),
                        },
                        {
                            key: 'delivery',
                            title: t('delivery'),
                            cell: ({ item: { delivery } }) => DeliveryRatio({ delivery }),
                        },
                        {
                            key: 'engagement',
                            title: t('engagement'),
                            cell: ({ item: { channel, delivery } }) => delivery?.opens > 0
                                ? (
                                    <div className="multi-cell no-image">
                                        <div className="text">
                                            <div className="title">
                                                {OpenRate({ delivery })} {t('open_rate')}
                                            </div>
                                            {channel === 'email' && <div className="subtitle">
                                                {ClickRate({ delivery })} {t('click_rate')}
                                            </div>}
                                        </div>
                                    </div>
                                )
                                : null,
                        },
                        {
                            key: 'send_at',
                            sortable: true,
                            title: t('launched_at'),
                            cell: ({ item: { send_at, type } }) => {
                                return send_at != null
                                    ? formatDate(preferences, send_at, 'Pp')
                                    : type === 'trigger'
                                        ? t('api_triggered')
                                        : <>&#8211;</>
                            },
                        },
                        {
                            key: 'updated_at',
                            title: t('updated_at'),
                            sortable: true,
                        },
                        {
                            key: 'options',
                            title: t('options'),
                            cell: ({ item: { id } }) => (
                                <Menu size="small">
                                    <MenuItem onClick={async () => await handleEditCampaign(id)}>
                                        <EditIcon />{t('edit')}
                                    </MenuItem>
                                    <MenuItem onClick={async () => await handleDuplicateCampaign(id)}>
                                        <DuplicateIcon />{t('duplicate')}
                                    </MenuItem>
                                    <MenuItem onClick={async () => await handleArchiveCampaign(id)}>
                                        <ArchiveIcon />{t('archive')}
                                    </MenuItem>
                                </Menu>
                            ),
                        },
                    ]}
                    onSelectRow={async ({ id }) => { await navigate(`/projects/${project.id}/campaigns/${id.toString()}`) }}
                    enableSearch
                    tagEntity="campaigns"
                    filters={[
                        <TypeSelect
                            key="type"
                            options={campaignTypes}
                            prefix={t('type')}
                            value={state.params.filter?.type || 'all'}
                            onChange={value => state.setParams({
                                ...state.params,
                                filter: {
                                    ...state.params.filter,
                                    type: value === 'all' ? '' : value,
                                },
                            })}
                            toValue={(value) => value.key}
                        />,
                    ]}
                />
            </PageContent>
        </>
    )
}
