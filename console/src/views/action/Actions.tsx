import { useCallback, useContext, useState, useRef } from 'react'
import { useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Plus, Search, ChevronLeft, ChevronRight, Zap, MoreHorizontal, Archive, Webhook } from 'lucide-react'

import api from '../../api'
import { useResolver } from '../../hooks'
import { formatDate, snakeToTitle } from '../../utils'
import { ProjectContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'

import type { Action } from '@/types'

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

interface ActionsProps {
    create?: boolean
}

export default function Actions({ create: _create = false }: ActionsProps) {
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
            return await api.actions.search(project.id, {
                limit: 25,
                cursor,
                page: pageDirection,
                search: debouncedQuery || undefined,
            })
        }, [project.id, debouncedQuery, cursor, pageDirection]),
    )

    const actions = result?.results
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

    const handleArchiveAction = async (e: React.MouseEvent, action: Action) => {
        e.stopPropagation()
        await api.actions.delete(project.id, action.id)
        await reload()
    }

    const handleRowClick = (action: Action) => {
        navigate(`/projects/${project.id}/actions/${action.id.toString()}`)
    }

    return (
        <div className="flex flex-col gap-6 p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <Zap />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">
                        {t('actions.plural')}
                    </h1>
                    <p className="text-sm text-muted-foreground">
                        {t('actions_description', 'Create and manage webhook actions and other integrations.')}
                    </p>
                </div>
            </div>

            {/* Search and Actions */}
            <div className="flex items-center justify-between gap-4">
                <div className="relative max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t('search_actions', 'Search actions...')}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <Button
                    size="lg"
                    onClick={() => navigate(`/projects/${project.id}/actions/new`)}
                >
                    <Plus className="mr-2 h-4 w-4" />
                    {t('create_action', 'Create Action')}
                </Button>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t('name')}</TableHead>
                            <TableHead>{t('type')}</TableHead>
                            <TableHead>{t('updated_at')}</TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!actions ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-md" />
                                            <Skeleton className="h-4 w-36" />
                                        </div>
                                    </TableCell>
                                    <TableCell><Skeleton className="h-5 w-16 rounded-md" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                                </TableRow>
                            ))
                        ) : actions.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Zap className="h-8 w-8" />
                                        <p>{debouncedQuery ? t('no_results') : t('no_actions_yet', 'No actions yet')}</p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => navigate(`/projects/${project.id}/actions/new`)}
                                                className="mt-2"
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t('create_action', 'Create Action')}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            actions.map((action) => (
                                <TableRow
                                    key={action.id}
                                    className="cursor-pointer"
                                    onClick={() => handleRowClick(action)}
                                >
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <div className="flex h-8 w-8 items-center justify-center rounded-md shrink-0 bg-yellow-50">
                                                <Webhook className="h-4 w-4 text-yellow-600" />
                                            </div>
                                            <span className="font-medium">{action.name}</span>
                                        </div>
                                    </TableCell>
                                    <TableCell>
                                        <Badge variant="secondary">{snakeToTitle(action.type)}</Badge>
                                    </TableCell>
                                    <TableCell className="text-muted-foreground">
                                        {formatDate(preferences, action.updated_at, 'PP')}
                                    </TableCell>
                                    <TableCell>
                                        <DropdownMenu>
                                            <DropdownMenuTrigger asChild>
                                                <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={(e) => e.stopPropagation()}>
                                                    <MoreHorizontal className="h-4 w-4" />
                                                </Button>
                                            </DropdownMenuTrigger>
                                            <DropdownMenuContent align="end">
                                                <DropdownMenuItem onClick={(e) => { e.stopPropagation(); handleRowClick(action) }}>
                                                    {t('edit')}
                                                </DropdownMenuItem>
                                                <DropdownMenuItem onClick={(e) => handleArchiveAction(e, action)} className="text-destructive">
                                                    <Archive className="mr-2 h-4 w-4" />
                                                    {t('delete')}
                                                </DropdownMenuItem>
                                            </DropdownMenuContent>
                                        </DropdownMenu>
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {actions && actions.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {actions.length} {t('actions.plural')}
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
        </div>
    )
}
