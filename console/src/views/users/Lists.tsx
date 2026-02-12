import type { Key, ReactNode } from 'react'
import type { List, ListState, SearchParams, SearchResult } from '../../types'
import { useSearchTableQueryState } from '../../ui/SearchTable'
import { Badge } from '@/components/ui/badge'
import { snakeToTitle, formatDate } from '../../utils'
import { useRoute } from '../router'
import api from '../../api'
import { useNavigate, useParams } from 'react-router'
import { Translation, useTranslation } from 'react-i18next'
import type { UUID } from '@/types/common'
import { NIL } from 'uuid'
import { useCallback, useContext, useState } from 'react'
import { CardContent } from '@/components/ui/card'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationNext,
    PaginationPrevious,
} from '@/components/ui/pagination'
import { Input } from '@/components/ui/input'
import { Search, MoreHorizontal } from 'lucide-react'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuGroup,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useDebounceControl } from '../../hooks'
import { Skeleton } from '@/components/ui/skeleton'
import { PreferencesContext } from '../../ui/PreferencesContext'
import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { PlusIcon } from '../../components/icons'
import { ListCreateForm } from './ListCreateForm'

interface ListTableParams {
    search: (params: SearchParams) => Promise<SearchResult<List>>
    title?: ReactNode
    selectedRow?: Key
    onSelectRow?: (list: List) => void
}

export const ListTag = ({ state, progress }: Pick<List, 'state' | 'progress'>) => {
    const variant: Record<ListState, 'default' | 'secondary' | 'destructive' | 'outline'> = {
        draft: 'outline',
        loading: 'secondary',
        ready: 'default',
    }

    const complete = progress?.complete ?? 0
    const total = progress?.total ?? 0
    const percent = total > 0 ? complete / total : 0
    const percentStr = percent.toLocaleString(undefined, { style: 'percent', minimumFractionDigits: 0 })

    return (
        <Badge variant={variant[state]}>
            <Translation>{(t) => t(state)}</Translation>
            {progress && ` (${percentStr})`}
        </Badge>
    )
}

function ListTable({ search, selectedRow, onSelectRow, title }: ListTableParams) {
    const route = useRoute()
    const { t } = useTranslation()
    const navigate = useNavigate()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [preferences] = useContext(PreferencesContext)
    const [isCreateListOpen, setIsCreateListOpen] = useState(false)

    function handleOnSelectRow(list: List) {
        if (onSelectRow) {
            onSelectRow(list)
        } else {
            route(`lists/${list.id}`)
        }
    }

    const handleDuplicateList = async (id: UUID) => {
        const list = await api.lists.duplicate(projectId, id)
        await navigate(list.id.toString())
    }

    const handleArchiveList = async (id: UUID) => {
        await api.lists.delete(projectId, id)
        await state.reload()
    }

    const state = useSearchTableQueryState(useCallback(async (params) => await search(params), [search]))
    const [searchValue, setSearchValue] = useDebounceControl(state.params.q ?? '', q => state.setParams({ ...state.params, q }))

    return <>
        <div className="py-8 px-8 space-y-8">
            <div className="flex items-center justify-between">
                <div className="space-y-1">
                    <h2 className="text-3xl font-bold tracking-tight">{title ?? t('lists')}</h2>
                </div>
                <div className="flex items-center gap-2">
                    <Button onClick={() => setIsCreateListOpen(true)}>
                        <PlusIcon />
                        {t('create_list')}
                    </Button>
                </div>
            </div>
            
            <div className="space-y-4">
                <div className="relative w-full max-w-sm">
                    <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                        type="search"
                        placeholder={t('search')}
                        value={searchValue}
                        onChange={(e) => setSearchValue(e.target.value)}
                        className="pl-8"
                    />
                </div>
                <div className="rounded-md border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t('name')}</TableHead>
                                <TableHead>{t('type')}</TableHead>
                                <TableHead>{t('users_count')}</TableHead>
                                <TableHead>{t('created_at')}</TableHead>
                                <TableHead>{t('updated_at')}</TableHead>
                                <TableHead>{t('options')}</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {state.results === null ? (
                                Array.from({ length: 5 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell></TableCell>
                                    </TableRow>
                                ))
                            ) : state.results.results.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={6} className="h-24 text-center">
                                        {t('no_results')}
                                    </TableCell>
                                </TableRow>
                            ) : (
                                state.results.results.map((list: List) => (
                                    <TableRow
                                        key={list.id}
                                        className={selectedRow === list.id ? 'bg-muted' : 'cursor-pointer'}
                                        onClick={() => handleOnSelectRow(list)}
                                        tabIndex={0}
                                        role="button"
                                        onKeyDown={(event) => {
                                            if (event.key === 'Enter' || event.key === ' ') {
                                                event.preventDefault()
                                                handleOnSelectRow(list)
                                            }
                                        }}
                                    >
                                        <TableCell className="font-medium">{list.name}</TableCell>
                                        <TableCell>{snakeToTitle(list.type)}</TableCell>
                                        <TableCell>{list.users_count?.toLocaleString()}</TableCell>
                                        <TableCell>{list.created_at ? formatDate(preferences, list.created_at) : '-'}</TableCell>
                                        <TableCell>{list.updated_at ? formatDate(preferences, list.updated_at) : '-'}</TableCell>
                                        <TableCell onClick={e => e.stopPropagation()}>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button
                                                        variant="ghost"
                                                        className="h-8 w-8 p-0"
                                                        aria-label={t('action')}
                                                    >
                                                        <MoreHorizontal />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="start" side="right">
                                                    <DropdownMenuGroup>
                                                        <DropdownMenuLabel>{t('action')}</DropdownMenuLabel>
                                                        <DropdownMenuItem onClick={() => route(`lists/${list.id}`)}>
                                                            {t('edit')}
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem onClick={() => handleDuplicateList(list.id)}>
                                                            {t('duplicate')}
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem
                                                            className="text-destructive"
                                                            onClick={() => handleArchiveList(list.id)}
                                                        >
                                                            {t('archive')}
                                                        </DropdownMenuItem>
                                                    </DropdownMenuGroup>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>
                {state.results && (
                    <Pagination className="mt-4">
                        <PaginationContent>
                            <PaginationItem>
                                <PaginationPrevious
                                    href="#"
                                    onClick={(e) => {
                                        e.preventDefault()
                                        state.setParams({ ...state.params, cursor: state.results!.prevCursor, page: 'prev' })
                                    }}
                                    aria-disabled={!state.results.prevCursor}
                                    className={!state.results.prevCursor ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                                />
                            </PaginationItem>
                            <PaginationItem>
                                <PaginationNext
                                    href="#"
                                    onClick={(e) => {
                                        e.preventDefault()
                                        state.setParams({ ...state.params, cursor: state.results!.nextCursor, page: 'next' })
                                    }}
                                    aria-disabled={!state.results.nextCursor}
                                    className={!state.results.nextCursor ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                                />
                            </PaginationItem>
                        </PaginationContent>
                    </Pagination>
                )}
            </div>
        </div>

        <Dialog open={isCreateListOpen} onOpenChange={setIsCreateListOpen}>
            <DialogContent className="max-w-lg">
                <DialogHeader>
                    <DialogTitle>{t('create_list')}</DialogTitle>
                </DialogHeader>
                <ListCreateForm
                    onCreated={async () => {
                        setIsCreateListOpen(false)
                        await state.reload()
                    }}
                />
            </DialogContent>
        </Dialog>
    </>
}

export default function Lists() {
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const search = useCallback(async (params: SearchParams) => await api.lists.search(projectId, params), [projectId])

    return (
        <CardContent>
            <ListTable search={search} />
        </CardContent>
    )
}
