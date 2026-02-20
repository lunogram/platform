import { useCallback, useContext, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Search } from 'lucide-react'
import api from '../../api'
import { ProjectContext, UserContext } from '../../contexts'
import { useDebounceControl, useResolver } from '../../hooks'
import type { SearchParams, UserEvent } from '../../types'
import { formatDate } from '../../utils'
import { PreferencesContext } from '../../ui/PreferencesContext'
import JsonPreview from '../../ui/JsonPreview'
import Iframe from '../../ui/Iframe'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
} from '@/components/ui/dialog'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationPrevious,
    PaginationNext,
} from '@/components/ui/pagination'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function UserDetailEvents() {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const [params, setParams] = useState<SearchParams>({
        limit: 25,
        q: '',
    })
    const [search, setSearch] = useDebounceControl(params.q ?? '', q => setParams({ ...params, q }))
    const projectId = project.id
    const userId = user.id
    const [results] = useResolver(useCallback(async () => await api.users.events(projectId, userId, params), [projectId, userId, params]))
    const [event, setEvent] = useState<UserEvent>()
    const hasPreview = !!event?.data?.result?.message?.html

    return (
        <div className="space-y-4">
            <Card>
                <CardHeader>
                    <CardTitle>{t('events')}</CardTitle>
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
                                    <TableHead>{t('name')}</TableHead>
                                    <TableHead>{t('created_at')}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {!results ? (
                                    Array.from({ length: 3 }).map((_, i) => (
                                        <TableRow key={i}>
                                            <TableCell colSpan={2}>
                                                <div className="h-4 w-3/4 animate-pulse rounded bg-muted" />
                                            </TableCell>
                                        </TableRow>
                                    ))
                                ) : results.results.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={2} className="text-center text-muted-foreground">
                                            {t('No Results')}
                                        </TableCell>
                                    </TableRow>
                                ) : (
                                    results.results.map((item) => (
                                        <TableRow
                                            key={item.id}
                                            className="cursor-pointer"
                                            role="button"
                                            tabIndex={0}
                                            onClick={() => setEvent(item)}
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter' || e.key === ' ') {
                                                    e.preventDefault()
                                                    setEvent(item)
                                                }
                                            }}
                                        >
                                            <TableCell>{item.name}</TableCell>
                                            <TableCell>{formatDate(preferences, item.created_at)}</TableCell>
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
                                            onClick={(e) => {
                                                e.preventDefault()
                                                setParams({ ...params, cursor: results.prevCursor!, page: 'prev' })
                                            }}
                                        />
                                    </PaginationItem>
                                )}
                                {results.nextCursor && (
                                    <PaginationItem>
                                        <PaginationNext
                                            href="#"
                                            onClick={(e) => {
                                                e.preventDefault()
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

            <Dialog open={!!event} onOpenChange={(open) => !open && setEvent(undefined)}>
                <DialogContent className={hasPreview ? 'max-w-[95vw] w-[95vw] h-[90vh]' : 'max-w-3xl'}>
                    <DialogHeader>
                        <DialogTitle>{event?.name}</DialogTitle>
                        {event && !hasPreview && (
                            <DialogDescription>
                                {formatDate(preferences, event.created_at)}
                            </DialogDescription>
                        )}
                    </DialogHeader>

                    {event && (
                        hasPreview ? (
                            <div className="grid grid-cols-2 gap-4 h-full overflow-hidden">
                                <div className="space-y-4 overflow-auto p-4">
                                    <p className="text-sm text-muted-foreground">
                                        {formatDate(preferences, event.created_at)}
                                    </p>
                                    <JsonPreview value={{ name: event.name, ...event.data, created_at: event.created_at }} />
                                </div>
                                <div className="h-full overflow-hidden">
                                    {event.name === 'email_sent' && event.data?.result?.message?.html && (
                                        <Iframe
                                            content={event.data.result.message.html}
                                            fullHeight={true}
                                            width="100%"
                                        />
                                    )}
                                </div>
                            </div>
                        ) : (
                            <div className="py-4">
                                <JsonPreview value={{ name: event.name, ...event.data, created_at: event.created_at }} />
                            </div>
                        )
                    )}
                </DialogContent>
            </Dialog>
        </div>
    )
}
