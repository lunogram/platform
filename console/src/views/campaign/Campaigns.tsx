import { useCallback, useContext } from 'react'
import { Link, useNavigate } from 'react-router'
import { MoreHorizontal, Search } from 'lucide-react'
import api from '../../api'
import { Button } from '@/components/ui/button'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import PageContent from '../../ui/PageContent'
import { useSearchTableQueryState } from '../../ui/SearchTable'
import { formatDate, snakeToTitle } from '../../utils'
import { ChannelIcon } from './ChannelTag'
import { Alert } from '../../ui'
import { ProjectContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'
import { useTranslation } from 'react-i18next'
import { useDebounceControl } from '../../hooks'
import CursorPagination from '../../ui/Pagination'
import { CreateCampaign } from './CreateCampaign'
import type { Campaign } from '@/types'
import type { UUID } from '@/types/common'

interface CampaignsProps {
    create?: boolean
}

export default function Campaigns({ create = false }: CampaignsProps) {
    const [userPrefs] = useContext(PreferencesContext)
    const [activeProject] = useContext(ProjectContext)
    const routerNavigate = useNavigate()
    const { t } = useTranslation()

    const tableState = useSearchTableQueryState(
        useCallback(queryParams => api.campaigns.search(activeProject.id, queryParams), [activeProject.id]),
    )

    const [searchQuery, updateSearchQuery] = useDebounceControl(
        tableState.params.q ?? '', 
        newQuery => tableState.setParams({ ...tableState.params, q: newQuery })
    )

    const navigateToEdit = (campaignId: UUID) => {
        routerNavigate(`/projects/${activeProject.id}/campaigns/${campaignId.toString()}`)
    }

    const duplicateCampaignAction = async (campaignId: UUID) => {
        try {
            const duplicated = await api.campaigns.duplicate(activeProject.id, campaignId)
            routerNavigate(`/projects/${activeProject.id}/campaigns/${duplicated.id.toString()}`)
        } catch (error) {
            console.error('Failed to duplicate campaign:', error)
        }
    }

    const archiveCampaignAction = async (campaignId: UUID) => {
        try {
            await api.campaigns.delete(activeProject.id, campaignId)
            await tableState.reload()
        } catch (error) {
            console.error('Failed to archive campaign:', error)
        }
    }

    const handleRowClick = (campaign: Campaign) => {
        routerNavigate(`/projects/${activeProject.id}/campaigns/${campaign.id.toString()}`)
    }

    const campaignsList = tableState.results?.results ?? []
    const isLoadingData = !tableState.results

    return (
        <>
            <PageContent 
                title={t('campaign.plural')} 
                actions={<CreateCampaign open={create} />} 
                banner={activeProject.has_provider === false && (
                    <Alert
                        variant="plain"
                        title={t('setup')}
                        actions={
                            <Link to={`/projects/${activeProject.id}/settings/integrations`}>
                                <Button>{t('setup_integration')}</Button>
                            </Link>
                        }
                    >{t('setup_integration_description')}</Alert>
                )}
            >
                <div className="flex flex-col gap-4">
                    <div className="flex items-center gap-2">
                        <div className="relative flex-1 max-w-sm">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder={t('search')}
                                value={searchQuery}
                                onChange={e => updateSearchQuery(e.target.value)}
                                className="pl-9"
                            />
                        </div>
                    </div>

                    <div className="rounded-md border">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t('name')}</TableHead>
                                    <TableHead>{t('updated_at')}</TableHead>
                                    <TableHead className="w-[50px]"></TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {isLoadingData ? (
                                    <TableRow>
                                        <TableCell colSpan={3} className="h-24 text-center">
                                            {t('loading')}
                                        </TableCell>
                                    </TableRow>
                                ) : campaignsList.length > 0 ? (
                                    campaignsList.map((campaign) => (
                                        <TableRow
                                            key={campaign.id}
                                            className="cursor-pointer"
                                            onClick={() => handleRowClick(campaign)}
                                        >
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div className="flex h-8 w-8 items-center justify-center rounded bg-muted">
                                                        <ChannelIcon channel={campaign.channel} />
                                                    </div>
                                                    <div className="flex flex-col">
                                                        <span className="font-medium">{campaign.name}</span>
                                                        <span className="text-sm text-muted-foreground">
                                                            {snakeToTitle(campaign.channel)}
                                                        </span>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                {formatDate(userPrefs, campaign.updated_at, 'Pp')}
                                            </TableCell>
                                            <TableCell onClick={e => e.stopPropagation()}>
                                                <DropdownMenu>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button variant="ghost" size="icon">
                                                            <MoreHorizontal className="h-4 w-4" />
                                                            <span className="sr-only">{t('options')}</span>
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        <DropdownMenuItem onClick={() => navigateToEdit(campaign.id)}>
                                                            {t('edit')}
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem onClick={() => duplicateCampaignAction(campaign.id)}>
                                                            {t('duplicate')}
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem onClick={() => archiveCampaignAction(campaign.id)}>
                                                            {t('archive')}
                                                        </DropdownMenuItem>
                                                    </DropdownMenuContent>
                                                </DropdownMenu>
                                            </TableCell>
                                        </TableRow>
                                    ))
                                ) : (
                                    <TableRow>
                                        <TableCell colSpan={3} className="h-24 text-center">
                                            {t('no_campaigns_found')}
                                        </TableCell>
                                    </TableRow>
                                )}
                            </TableBody>
                        </Table>
                    </div>

                    {tableState.results && (
                        <CursorPagination
                            nextCursor={tableState.results.nextCursor}
                            prevCursor={tableState.results.prevCursor}
                            onPrev={cursor => tableState.setParams({ ...tableState.params, cursor, page: 'prev' })}
                            onNext={cursor => tableState.setParams({ ...tableState.params, cursor, page: 'next' })}
                        />
                    )}
                </div>
            </PageContent>
        </>
    )
}
