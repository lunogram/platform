import { useCallback, useContext } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { Search } from 'lucide-react'
import api from '../../api'
import { ProjectContext, UserContext } from '../../contexts'
import { useDebounceControl } from '../../hooks'
import type { JourneyUserStep } from '../../types'
import { useSearchTableQueryState } from '../../ui/SearchTable'
import { formatDate } from '../../utils'
import { PreferencesContext } from '../../ui/PreferencesContext'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationPrevious,
    PaginationNext,
} from '@/components/ui/pagination'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function UserDetailJourneys() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const [preferences] = useContext(PreferencesContext)

    const projectId = project.id
    const userId = user.id

    const { results, params, setParams } = useSearchTableQueryState(
        useCallback(async (params) => await api.users.journeys.search(projectId, userId, params), [projectId, userId])
    )

    const [search, setSearch] = useDebounceControl(params.q ?? '', q => setParams({ ...params, q }))

    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('journeys')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
                <div className="relative w-full max-w-sm">
                    <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                        type="search"
                        placeholder={t('search')}
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="pl-8"
                    />
                </div>

                <div className="rounded-md border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t('journey')}</TableHead>
                                <TableHead>{t('created_at')}</TableHead>
                                <TableHead>{t('ended_at')}</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {!results ? (
                                Array.from({ length: 3 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell colSpan={3}>
                                            <div className="h-4 w-3/4 animate-pulse rounded bg-muted" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : results.results.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={3} className="text-center text-muted-foreground">
                                        {t('campaign.setup.channels.email.no_content_available')}
                                    </TableCell>
                                </TableRow>
                            ) : (
                                results.results.map((item: JourneyUserStep) => (
                                    <TableRow
                                        key={item.id}
                                                                                className="cursor-pointer"
                                        role="button"
                                        tabIndex={0}
                                        onClick={() => navigate(`../../entrances/${item.entrance_id}`)}
                                        onKeyDown={(event) => {
                                            if (event.key === 'Enter' || event.key === ' ') {
                                                event.preventDefault()
                                                navigate(`../../entrances/${item.entrance_id}`)
                                            }
                                        }}
                                        >
                                        <TableCell>{item.journey?.name}</TableCell>
                                        <TableCell>{formatDate(preferences, item.created_at)}</TableCell>
                                        <TableCell>
                                            {item.ended_at
                                                ? formatDate(preferences, item.ended_at, 'Ppp')
                                                : <Badge variant="secondary">{t('running')}</Badge>
                                            }
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>

                {results && (
                    <Pagination>
                        <PaginationContent>
                            {results.prevCursor && (
                                <PaginationItem>
                                    <PaginationPrevious
                                        href="#"
                                        onClick={(event) => {
                                            event.preventDefault()
                                            setParams({ ...params, cursor: results.prevCursor!, page: 'prev' })
                                        }}
                                    />
                                </PaginationItem>
                            )}
                            {results.nextCursor && (
                                <PaginationItem>
                                    <PaginationNext
                                        href="#"
                                        onClick={(event) => {
                                            event.preventDefault()
                                            setParams({ ...params, cursor: results.nextCursor!, page: 'next' })
                                        }}
                                    />
                                </PaginationItem>
                            )}
                        </PaginationContent>
                    </Pagination>
                )}
            </CardContent>
        </Card>
    )
}
