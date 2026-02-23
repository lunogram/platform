import { useCallback, useState } from 'react'
import { useParams } from 'react-router'
import { CreateUserForm } from './CreateUserForm'
import { BulkRemoveUsersForm } from './BulkRemoveUsersForm'
import api from '../../api'
import { useSearchTableQueryState } from '../../ui/SearchTable'
import { useRoute } from '../router'
import { useDebounceControl } from '../../hooks'
import { Input } from '@/components/ui/input'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationNext,
    PaginationPrevious,
} from '@/components/ui/pagination'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { PlusIcon, TrashIcon } from '../../components/icons'
import type { UUID } from '@/types/common'
import { NIL } from 'uuid'
import type { User } from '../../types'

import './Users.css'

// eslint-disable-next-line @typescript-eslint/no-namespace
export declare namespace Intl {
    type Key = 'calendar' | 'collation' | 'currency' | 'numberingSystem' | 'timeZone' | 'unit'
    function supportedValuesOf(input: Key): string[]

    interface DateTimeFormat {
        format(date?: Date | number): string
        resolvedOptions(): ResolvedDateTimeFormatOptions
    }

    interface ResolvedDateTimeFormatOptions {
        locale: string
        timeZone: string
        timeZoneName?: string
    }

    // eslint-disable-next-line no-var
    var DateTimeFormat: {
        new(locales?: string | string[]): DateTimeFormat
        (locales?: string | string[]): DateTimeFormat
    }
}

export default function UserTabs() {
    const { projectId = NIL } = useParams<{ projectId: UUID }>()
    const { t } = useTranslation()
    const route = useRoute()
    const timeZones = Intl.supportedValuesOf('timeZone')
    const locale = navigator.languages[0]?.split('-')[0] ?? 'en'

    const state = useSearchTableQueryState(useCallback(async params => { return await api.users.search(projectId, params) }, [projectId]))
    const [isBulkRemovalOpen, setIsBulkRemovalOpen] = useState(false)
    const [isCreateUserOpen, setIsCreateUserOpen] = useState(false)
    const [search, setSearch] = useDebounceControl(state.params.q ?? '', q => state.setParams({ ...state.params, q }))

    const defaultUser: Pick<User, 'timezone' | 'locale' | 'data'> = {
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        locale,
        data: {},
    }

    const createUser = async (user: User) => {
        const { full_name, ...rest } = user
        const newUser: User = {
            ...rest,
            anonymous_id: crypto.randomUUID() as UUID,
            email: user.email || undefined,
            phone: user.phone || undefined,
            ...(full_name
                ? { data: { full_name } }
                : { data: user.data || {} }),
        }

        await api.users.create(projectId, newUser)
        await state.reload()

        setIsCreateUserOpen(false)
    }

    const bulkRemoveUsers = async (file: FileList) => {
        await api.users.deleteImport(projectId, file[0])
        await state.reload()
        setIsBulkRemovalOpen(false)
    }

    return (
        <>
            <div className="py-8 px-8 space-y-8">
                <div className="flex items-center justify-between">
                    <div className="space-y-1">
                        <h2 className="text-3xl font-bold tracking-tight">{t('users')}</h2>
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            onClick={() => setIsBulkRemovalOpen(true)}
                            variant="destructive">
                            <TrashIcon />
                            {t('delete_users')}
                        </Button>
                        <Button onClick={() => setIsCreateUserOpen(true)}>
                            <PlusIcon />
                            {t('create_user')}
                        </Button>
                    </div>
                </div>

                <div className="space-y-4">
                    <Input
                        type="search"
                        placeholder={t('search_users')}
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="max-w-sm"
                    />
                    <div className="rounded-md border">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t('name')}</TableHead>
                                    <TableHead>{t('external_id')}</TableHead>
                                    <TableHead>{t('email')}</TableHead>
                                    <TableHead>{t('phone')}</TableHead>
                                    <TableHead>{t('locale.singular')}</TableHead>
                                    <TableHead
                                        className="cursor-pointer select-none"
                                        onClick={() =>
                                            state.setParams({
                                                ...state.params,
                                                sort: 'name',
                                                direction:
                                                    state.params.sort === 'name' && state.params.direction === 'asc'
                                                        ? 'desc'
                                                        : 'asc',
                                            })
                                        }
                                    >
                                        {t('name')}
                                    </TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {state.results === null ? (
                                    Array.from({ length: 5 }).map((_, i) => (
                                        <TableRow key={i}>
                                            <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        </TableRow>
                                    ))
                                ) : state.results.results.length ? (
                                    state.results.results.map((user) => (
                                        <TableRow
                                            key={user.id}
                                            className="cursor-pointer"
                                            onClick={() => route(`users/${user.id}`)}
                                            tabIndex={0}
                                            role="button"
                                            onKeyDown={(event) => {
                                                if (event.key === 'Enter' || event.key === ' ') {
                                                    event.preventDefault()
                                                    route(`users/${user.id}`)
                                                }
                                            }}
                                        >
                                            <TableCell>{user.data?.full_name || user.full_name || '-'}</TableCell>
                                            <TableCell>{user.external_id || '-'}</TableCell>
                                            <TableCell>{user.email || '-'}</TableCell>
                                            <TableCell>{user.data?.phone || user.phone || '-'}</TableCell>
                                            <TableCell>{user.locale || '-'}</TableCell>
                                            <TableCell>{user.created_at ? new Date(user.created_at).toLocaleDateString() : '-'}</TableCell>
                                        </TableRow>
                                    ))
                                ) : (
                                    <TableRow>
                                        <TableCell colSpan={6} className="h-24 text-center">
                                            {t('campaign.setup.channels.email.no_content_available')}
                                        </TableCell>
                                    </TableRow>
                                )}
                            </TableBody>
                        </Table>
                    </div>
                    {state.results && (
                        <Pagination className="mt-4">
                            <PaginationContent>
                                {state.results.prevCursor && (
                                    <PaginationItem>
                                        <PaginationPrevious
                                            href="#"
                                            onClick={(e) => {
                                                e.preventDefault()
                                                state.setParams({ ...state.params, cursor: state.results!.prevCursor, page: 'prev' })
                                            }}
                                            className="cursor-pointer"
                                        />
                                    </PaginationItem>
                                )}
                                {state.results.nextCursor && (
                                    <PaginationItem>
                                        <PaginationNext
                                            href="#"
                                            onClick={(e) => {
                                                e.preventDefault()
                                                state.setParams({ ...state.params, cursor: state.results!.nextCursor, page: 'next' })
                                            }}
                                            className="cursor-pointer"
                                        />
                                    </PaginationItem>
                                )}
                            </PaginationContent>
                        </Pagination>
                    )}
                </div>
            </div>

            <Dialog open={isCreateUserOpen} onOpenChange={setIsCreateUserOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('create_user')}</DialogTitle>
                    </DialogHeader>
                    <CreateUserForm
                        defaultUser={defaultUser}
                        timeZones={timeZones}
                        onSubmit={createUser}
                    />
                </DialogContent>
            </Dialog>

            <Dialog open={isBulkRemovalOpen} onOpenChange={setIsBulkRemovalOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('delete_users')}</DialogTitle>
                    </DialogHeader>
                    <BulkRemoveUsersForm onSubmit={bulkRemoveUsers} />
                </DialogContent>
            </Dialog>
        </>
    )
}
