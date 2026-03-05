import { useCallback, useContext, useState, useRef } from 'react'
import { Link, useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Search, ChevronLeft, ChevronRight, ArrowRight, Megaphone, Mail, Smartphone, MessageSquareDot, MoreHorizontal, Copy, Archive } from 'lucide-react'

import api from '../../api'
import { useResolver } from '../../hooks'
import { formatDate, snakeToTitle } from '../../utils'
import { getRandomColor } from '@/lib/colors'
import { ProjectContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'
import { Alert } from '../../ui'
import { CampaignsIcon } from '@/components/icons'

import { CreateCampaign } from './CreateCampaign'

import type { Campaign, CampaignDelivery, CampaignState, ChannelType } from '@/types'
import type { UUID } from '@/types/common'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    text: Smartphone,
    push: MessageSquareDot,
}

function getStateBadge(state: CampaignState, t: (key: string) => string) {
    const config: Record<CampaignState, { label: string; className: string }> = {
        draft: { label: t('draft'), className: 'bg-secondary text-secondary-foreground' },
        loading: { label: t('loading'), className: 'bg-blue-100 text-blue-700' },
        scheduled: { label: t('scheduled'), className: 'bg-blue-100 text-blue-700' },
        running: { label: t('running'), className: 'bg-blue-100 text-blue-700' },
        finished: { label: t('finished'), className: 'bg-green-100 text-green-700' },
        aborting: { label: t('aborting'), className: 'bg-red-100 text-red-700' },
        aborted: { label: t('aborted'), className: 'bg-red-100 text-red-700' },
    }
    const { label, className } = config[state] ?? config.draft
    return <Badge variant="outline" className={`border-0 ${className}`}>{label}</Badge>
}

function formatDelivery(delivery: CampaignDelivery) {
    const sent = delivery?.sent ?? 0
    const total = delivery?.total ?? 0
    const ratio = total > 0 ? sent / total : 0
    return `${sent.toLocaleString()} (${ratio.toLocaleString(undefined, { style: 'percent', minimumFractionDigits: 0 })})`
}

interface CampaignsProps {
    create?: boolean
}

export default function Campaigns({ create = false }: CampaignsProps) {
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()

    const [searchQuery, setSearchQuery] = useState('')
    const [debouncedQuery, setDebouncedQuery] = useState('')
    const [cursor, setCursor] = useState<string | undefined>()
    const [pageDirection, setPageDirection] = useState<'next' | 'prev' | undefined>()
    const [cursorHistory, setCursorHistory] = useState<string[]>([])
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setCursor(undefined)
        setPageDirection(undefined)
        setCursorHistory([])
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.campaigns.search(project.id, {
                limit: 25,
                cursor,
                page: pageDirection,
                search: debouncedQuery || undefined,
            })
        }, [project.id, debouncedQuery, cursor, pageDirection]),
    )

    const campaigns = result?.results
    const hasNextPage = !!result?.nextCursor
    const hasPrevPage = cursorHistory.length > 0

    const handleNextPage = () => {
        if (result?.nextCursor) {
            setCursorHistory(prev => [...prev, cursor ?? ''])
            setCursor(result.nextCursor)
            setPageDirection('next')
        }
    }

    const handlePrevPage = () => {
        if (cursorHistory.length > 0) {
            const prev = [...cursorHistory]
            const prevCursor = prev.pop()
            setCursorHistory(prev)
            setCursor(prevCursor || undefined)
            setPageDirection(prevCursor ? 'next' : undefined)
        }
    }

    const handleDuplicateCampaign = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        const campaign = await api.campaigns.duplicate(project.id, id)
        await navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}`)
    }

    const handleArchiveCampaign = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        await api.campaigns.delete(project.id, id)
        await reload()
    }

    const handleRowClick = (campaign: Campaign) => {
        navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}`)
    }

    return (
        <div className="flex flex-col gap-6 p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <CampaignsIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">
                        {t('campaign.plural')}
                    </h1>
                    <p className="text-sm text-muted-foreground">
                        {t('campaigns_description', 'Create and manage email, SMS, and push notification campaigns to engage your audience.')}
                    </p>
                </div>
            </div>

            {/* Provider setup banner */}
            {project.has_provider === false && (
                <Alert
                    variant="plain"
                    title={t('setup')}
                    actions={
                        <Link to={`/projects/${project.id}/settings/integrations`}>
                            <Button>{t('setup_integration')}</Button>
                        </Link>
                    }
                >{t('setup_integration_description')}</Alert>
            )}

            {/* Search and Actions */}
            <div className="flex items-center justify-between gap-4">
                <div className="relative max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t('search_campaigns', 'Search campaigns...')}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <CreateCampaign open={create} />
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t('name')}</TableHead>
                            <TableHead>{t('state')}</TableHead>
                            <TableHead>{t('delivery')}</TableHead>
                            <TableHead>{t('launched_at')}</TableHead>
                            <TableHead>{t('updated_at')}</TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!campaigns ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-md" />
                                            <div className="space-y-1">
                                                <Skeleton className="h-4 w-36" />
                                                <Skeleton className="h-3 w-20" />
                                            </div>
                                        </div>
                                    </TableCell>
                                    <TableCell><Skeleton className="h-5 w-16 rounded-md" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                                </TableRow>
                            ))
                        ) : campaigns.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={6} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Megaphone className="h-8 w-8" />
                                        <p>{debouncedQuery ? t('no_campaigns_found') : t('no_campaigns_yet', 'No campaigns yet')}</p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            campaigns.map((campaign) => {
                                const campaignColor = getRandomColor(campaign.name ?? campaign.id)
                                const ChannelIcon = channelIcons[campaign.channel] ?? Mail
                                return (
                                    <TableRow
                                        key={campaign.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(campaign)}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                    style={{ backgroundColor: campaignColor }}
                                                >
                                                    <ChannelIcon className="h-4 w-4 text-white" />
                                                </div>
                                                <div>
                                                    <div className="font-medium">{campaign.name}</div>
                                                    <div className="text-sm text-muted-foreground">
                                                        {snakeToTitle(campaign.channel)}
                                                    </div>
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            {getStateBadge(campaign.state, t)}
                                        </TableCell>
                                        <TableCell className="text-muted-foreground">
                                            {campaign.delivery?.sent > 0
                                                ? formatDelivery(campaign.delivery)
                                                : '—'}
                                        </TableCell>
                                        <TableCell className="text-muted-foreground">
                                            {campaign.send_at
                                                ? formatDate(preferences, campaign.send_at, 'Pp')
                                                : campaign.type === 'trigger'
                                                    ? t('api_triggered')
                                                    : '—'}
                                        </TableCell>
                                        <TableCell className="text-muted-foreground">
                                            {formatDate(preferences, campaign.updated_at, 'PP')}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={(e) => e.stopPropagation()}>
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); handleRowClick(campaign) }}>
                                                        <Mail className="mr-2 h-4 w-4" />
                                                        {t('edit')}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={(e) => handleDuplicateCampaign(e, campaign.id)}>
                                                        <Copy className="mr-2 h-4 w-4" />
                                                        {t('duplicate')}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={(e) => handleArchiveCampaign(e, campaign.id)} className="text-destructive">
                                                        <Archive className="mr-2 h-4 w-4" />
                                                        {t('archive')}
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {campaigns && campaigns.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {campaigns.length} {t('campaign.plural').toLowerCase()}
                        </p>
                        {(hasPrevPage || hasNextPage) && (
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handlePrevPage}
                                    disabled={!hasPrevPage}
                                >
                                    <ChevronLeft className="h-4 w-4 mr-1" />
                                    {t('previous')}
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handleNextPage}
                                    disabled={!hasNextPage}
                                >
                                    {t('next')}
                                    <ChevronRight className="h-4 w-4 ml-1" />
                                </Button>
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Tip Card */}
            <div className="group relative overflow-hidden rounded-lg bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border p-6">
                <div className="relative z-10 max-w-md">
                    <h3 className="font-semibold text-foreground">
                        {t('campaign_tip_title', 'Automate campaigns via API')}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t('campaign_tip_description', 'Trigger campaigns programmatically using the API for automated messaging workflows.')}
                    </p>
                    <Button
                        variant="link"
                        className="mt-2 h-auto p-0 text-primary"
                        onClick={() => window.open('/api/', '_blank')}
                    >
                        {t('view_api_docs', 'View API documentation')}
                        <ArrowRight className="ml-1 h-3 w-3 transition-transform duration-300 group-hover:translate-x-1" />
                    </Button>
                </div>

                {/* Decorative elements */}
                <div className="absolute -right-6 -bottom-6 flex gap-4">
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 transition-all duration-500 ease-out group-hover:rotate-6 group-hover:-translate-y-2 group-hover:bg-primary/15">
                        <Megaphone className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110" strokeWidth={1.25} />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Mail className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110" strokeWidth={1.25} />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Smartphone className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110" strokeWidth={1.25} />
                    </div>
                </div>
            </div>
        </div>
    )
}


