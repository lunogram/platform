import { useCallback, useContext, useEffect, useState, useRef } from 'react'
import { Link, useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import {
    ListFilter,
    ChevronRight,
    ChevronLeft,
    MoreHorizontal,
    Send,
    Upload,
    Pencil,
    RefreshCw,
    Archive,
    Search,
    AlertCircle,
    Users,
} from 'lucide-react'
import api from '../../api'
import { ListContext, ProjectContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'
import type { DynamicList, ListUpdateParams, Rule, WrapperRule } from '../../types'
import { formatDate, snakeToTitle } from '../../utils'
import { getRandomColor } from '@/lib/colors'
import RuleBuilder from './rules/RuleBuilder'
import { useRoute } from '../router'
import { useBlocker } from 'react-router'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
    Alert,
    AlertDescription,
    AlertTitle,
} from '@/components/ui/alert'
import { Label } from '@/components/ui/label'

import type { ListState } from '../../types'

function getStateBadge(state: ListState, t: (key: string) => string) {
    const config: Record<ListState, { label: string; className: string }> = {
        draft: { label: t('draft'), className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
        loading: { label: t('loading'), className: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
        ready: { label: t('ready'), className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
    }
    const { label, className } = config[state] ?? config.draft
    return <Badge variant="outline" className={`border-0 ${className}`}>{label}</Badge>
}

interface RuleSectionProps {
    list: DynamicList
    isSaving: boolean
    onRuleSave: (rule: Rule) => void
    onChange?: (rule: Rule) => void
}

function RuleSection({ list, isSaving, onRuleSave, onChange }: RuleSectionProps) {
    const { t } = useTranslation()
    const [rule, setRule] = useState<Rule>(list.rule)
    const onSetRule = (rule: Rule) => {
        setRule(rule)
        onChange?.(rule)
    }
    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-base font-semibold">{t('rules')}</h3>
                    <p className="text-sm text-muted-foreground">
                        {t('rules_description', 'Define conditions to dynamically include users in this list.')}
                    </p>
                </div>
                <Button
                    size="sm"
                    onClick={() => onRuleSave(rule)}
                    disabled={isSaving}
                >
                    {isSaving ? (
                        <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" />
                    ) : null}
                    {t('rules_save')}
                </Button>
            </div>
            <div className="rounded-lg border bg-card p-4">
                <RuleBuilder rule={rule} setRule={onSetRule} />
            </div>
        </div>
    )
}

export default function ListDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)
    const [list, setList] = useContext(ListContext)
    const [isEditListOpen, setIsEditListOpen] = useState(false)
    const [isUploadOpen, setIsUploadOpen] = useState(false)
    const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)
    const [isSaving, setIsSaving] = useState(false)
    const [error, setError] = useState<string | undefined>()
    const [editName, setEditName] = useState(list.name)

    // Users table state
    const [users, setUsers] = useState<any[] | null>(null)
    const [searchQuery, setSearchQuery] = useState('')
    const [debouncedQuery, setDebouncedQuery] = useState('')
    const [cursor, setCursor] = useState<string | undefined>()
    const [pageDirection, setPageDirection] = useState<'next' | 'prev' | undefined>()
    const [cursorHistory, setCursorHistory] = useState<string[]>([])
    const [nextCursor, setNextCursor] = useState<string | undefined>()
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const route = useRoute()

    const listColor = getRandomColor(list.name ?? list.id)

    const loadUsers = useCallback(async () => {
        try {
            const result = await api.lists.users(project.id, list.id, {
                limit: 25,
                cursor,
                page: pageDirection,
                search: debouncedQuery || undefined,
            })
            setUsers(result.results)
            setNextCursor(result.nextCursor || undefined)
        } catch {
            setUsers([])
        }
    }, [project.id, list.id, cursor, pageDirection, debouncedQuery])

    useEffect(() => {
        loadUsers()
    }, [loadUsers])

    const refreshList = useCallback(() => {
        api.lists.get(project.id, list.id)
            .then(setList)
            .then(() => loadUsers())
            .catch(() => { })
    }, [project.id, list.id, setList, loadUsers])

    useEffect(() => {
        if (list.state !== 'loading') return
        const complete = list.progress?.complete ?? 0
        const total = list.progress?.total ?? 0
        const percent = total > 0 ? complete / total * 100 : 0
        const refreshRate = percent < 5 ? 1000 : 5000
        const interval = setInterval(refreshList, refreshRate)
        refreshList()

        return () => clearInterval(interval)
    }, [list.state, list.progress?.complete, list.progress?.total, refreshList])

    const blocker = useBlocker(
        ({ currentLocation, nextLocation }) => hasUnsavedChanges && currentLocation.pathname !== nextLocation.pathname,
    )

    useEffect(() => {
        if (blocker.state !== 'blocked') return
        if (confirm(t('confirm_unsaved_changes'))) {
            blocker.proceed()
        } else {
            blocker.reset()
        }
    }, [blocker, t])

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

    const hasPrevPage = cursorHistory.length > 0
    const hasNextPage = !!nextCursor

    const handleNextPage = () => {
        if (nextCursor) {
            setCursorHistory(prev => [...prev, cursor ?? ''])
            setCursor(nextCursor)
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

    const saveList = async ({ name, rule, published, tags }: ListUpdateParams) => {
        setIsSaving(true)
        try {
            const value = await api.lists.update(project.id, list.id, { name, rule, published, tags })
            setError(undefined)
            setList(value)
            setIsEditListOpen(false)
            setHasUnsavedChanges(false)
        } catch (error: unknown) {
            const errorMessage = error instanceof Error ? error.message : 'An unexpected error occurred'
            setError(errorMessage)
            setIsEditListOpen(false)
        } finally {
            setIsSaving(false)
        }
    }

    const uploadUsers = async (file: File) => {
        await api.lists.upload(project.id, list.id, file)
        refreshList()
        setIsUploadOpen(false)
    }

    const handleRecountList = async () => {
        await api.lists.recount(project.id, list.id)
        window.location.reload()
    }

    const handleArchiveList = async () => {
        await api.lists.delete(project.id, list.id)
        await navigate(`/projects/${project.id}/lists`)
    }

    const progress = list.state === 'loading' && list.progress
        ? Math.round((list.progress.complete / (list.progress.total || 1)) * 100)
        : null

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section */}
            <div className="border-b bg-card/50">
                <div className="p-6">
                    {/* Breadcrumb */}
                    <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                        <Link
                            to={`/projects/${project.id}/lists`}
                            className="hover:text-foreground transition-colors"
                        >
                            {t('lists')}
                        </Link>
                        <ChevronRight className="h-3.5 w-3.5" />
                        <span className="text-foreground font-medium">
                            {list.name}
                        </span>
                    </nav>

                    {/* List Identity */}
                    <div className="flex items-start justify-between gap-6">
                        <div className="flex items-start gap-4">
                            <div
                                className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0"
                                style={{ backgroundColor: listColor }}
                            >
                                <ListFilter className="h-7 w-7 text-white" />
                            </div>
                            <div className="space-y-1">
                                <div className="flex items-center gap-3">
                                    <h1 className="text-2xl font-semibold tracking-tight">
                                        {list.name}
                                    </h1>
                                    {getStateBadge(list.state, t)}
                                </div>
                                <p className="text-sm text-muted-foreground flex items-center gap-2">
                                    <span>{snakeToTitle(list.type)}</span>
                                    <span>·</span>
                                    <span>
                                        {list.state === 'loading'
                                            ? t('counting', 'Counting...')
                                            : `${list.users_count?.toLocaleString() ?? 0} ${t('users').toLowerCase()}`
                                        }
                                    </span>
                                    <span>·</span>
                                    <span>
                                        {t('created')} {formatDate(preferences, list.created_at, 'PP')}
                                    </span>
                                </p>
                            </div>
                        </div>

                        <div className="flex items-center gap-2">
                            {list.state === 'draft' && (
                                <Button
                                    size="sm"
                                    onClick={async () => await saveList({ name: list.name, published: true })}
                                >
                                    <Send className="mr-2 h-3.5 w-3.5" />
                                    {t('publish')}
                                </Button>
                            )}
                            {list.type === 'static' && (
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => setIsUploadOpen(true)}
                                >
                                    <Upload className="mr-2 h-3.5 w-3.5" />
                                    {t('upload_list')}
                                </Button>
                            )}
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => {
                                    setEditName(list.name)
                                    setIsEditListOpen(true)
                                }}
                            >
                                <Pencil className="mr-2 h-3.5 w-3.5" />
                                {t('edit_list')}
                            </Button>
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-8 w-8">
                                        <MoreHorizontal className="h-4 w-4" />
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                    <DropdownMenuItem onClick={handleRecountList}>
                                        <RefreshCw className="h-4 w-4 mr-2" />
                                        {t('recount')}
                                    </DropdownMenuItem>
                                    <DropdownMenuSeparator />
                                    <DropdownMenuItem
                                        className="text-destructive focus:text-destructive"
                                        onClick={handleArchiveList}
                                    >
                                        <Archive className="h-4 w-4 mr-2" />
                                        {t('archive')}
                                    </DropdownMenuItem>
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </div>
                    </div>
                </div>
            </div>

            {/* Progress Bar for Loading State */}
            {list.state === 'loading' && progress !== null && (
                <div className="border-b bg-blue-50/50 dark:bg-blue-950/20 px-6 py-3">
                    <div className="flex items-center gap-3">
                        <RefreshCw className="h-4 w-4 animate-spin text-blue-600 dark:text-blue-400" />
                        <div className="flex-1">
                            <div className="flex items-center justify-between text-sm mb-1">
                                <span className="text-blue-700 dark:text-blue-300 font-medium">
                                    {t('processing', 'Processing...')}
                                </span>
                                <span className="text-blue-600 dark:text-blue-400">{progress}%</span>
                            </div>
                            <div className="h-1.5 rounded-full bg-blue-200/60 dark:bg-blue-800/40 overflow-hidden">
                                <div
                                    className="h-full rounded-full bg-blue-600 dark:bg-blue-400 transition-all duration-500"
                                    style={{ width: `${progress}%` }}
                                />
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Content Area */}
            <div className="flex-1 p-6 space-y-6">
                {/* Error Alert */}
                {error && (
                    <Alert variant="destructive">
                        <AlertCircle className="h-4 w-4" />
                        <AlertTitle>{t('error')}</AlertTitle>
                        <AlertDescription>{error}</AlertDescription>
                    </Alert>
                )}

                {/* Rules Section (Dynamic Lists) */}
                {list.type === 'dynamic' && (
                    <RuleSection
                        list={list}
                        isSaving={isSaving}
                        onRuleSave={async (rule) => await saveList({ name: list.name, rule: rule as WrapperRule })}
                        onChange={() => setHasUnsavedChanges(true)}
                    />
                )}

                {/* Users Table */}
                <div className="space-y-4">
                    <div className="flex items-center justify-between gap-4">
                        <h3 className="text-base font-semibold">{t('users')}</h3>
                        <div className="relative max-w-xs flex-1">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder={t('search_users', 'Search users...')}
                                value={searchQuery}
                                onChange={(e) => handleSearch(e.target.value)}
                                className="pl-9 h-8"
                            />
                        </div>
                    </div>

                    <div className="rounded-lg border bg-card">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t('name')}</TableHead>
                                    <TableHead>{t('external_id')}</TableHead>
                                    <TableHead>{t('email')}</TableHead>
                                    <TableHead>{t('phone')}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {!users ? (
                                    Array.from({ length: 5 }).map((_, i) => (
                                        <TableRow key={i}>
                                            <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-36" /></TableCell>
                                            <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        </TableRow>
                                    ))
                                ) : users.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={4} className="h-32 text-center">
                                            <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                                <Users className="h-8 w-8" />
                                                <p>{debouncedQuery ? t('no_users_found', 'No users found') : t('no_users_yet', 'No users in this list yet')}</p>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ) : (
                                    users.map((user: any) => (
                                        <TableRow
                                            key={user.id}
                                            className="cursor-pointer"
                                            onClick={() => route(`users/${user.id}`)}
                                        >
                                            <TableCell className="font-medium">
                                                {user.full_name || '—'}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                                    {user.external_id}
                                                </code>
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                {user.email || '—'}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                {user.phone || '—'}
                                            </TableCell>
                                        </TableRow>
                                    ))
                                )}
                            </TableBody>
                        </Table>

                        {/* Pagination */}
                        {users && users.length > 0 && (
                            <div className="flex items-center justify-between border-t px-4 py-3">
                                <p className="text-sm text-muted-foreground">
                                    {users.length} {t('users').toLowerCase()}
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
            </div>

            {/* Edit List Dialog */}
            <Dialog open={isEditListOpen} onOpenChange={setIsEditListOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('edit_list')}</DialogTitle>
                        <DialogDescription>
                            {t('edit_list_description', 'Update the list name and settings.')}
                        </DialogDescription>
                    </DialogHeader>
                    <form onSubmit={async (e) => {
                        e.preventDefault()
                        await saveList({ name: editName })
                    }}>
                        <div className="grid gap-4 py-4">
                            <div className="grid gap-2">
                                <Label htmlFor="list-name">{t('list_name')}</Label>
                                <Input
                                    id="list-name"
                                    value={editName}
                                    onChange={(e) => setEditName(e.target.value)}
                                    required
                                />
                            </div>
                        </div>
                        <DialogFooter>
                            <Button
                                type="button"
                                variant="outline"
                                onClick={() => setIsEditListOpen(false)}
                                disabled={isSaving}
                            >
                                {t('cancel')}
                            </Button>
                            <Button type="submit" disabled={isSaving}>
                                {isSaving ? t('saving', 'Saving...') : t('save')}
                            </Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>

            {/* Upload Users Dialog */}
            <Dialog open={isUploadOpen} onOpenChange={setIsUploadOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('import_users')}</DialogTitle>
                        <DialogDescription>
                            {t('upload_instructions')}
                        </DialogDescription>
                    </DialogHeader>
                    <form onSubmit={async (e) => {
                        e.preventDefault()
                        const formData = new FormData(e.currentTarget)
                        const file = formData.get('file') as File
                        if (file) await uploadUsers(file)
                    }}>
                        <div className="grid gap-4 py-4">
                            <div className="grid gap-2">
                                <Label htmlFor="upload-file">{t('file')}</Label>
                                <Input
                                    id="upload-file"
                                    name="file"
                                    type="file"
                                    accept=".csv,.txt"
                                    required
                                    className="cursor-pointer"
                                />
                            </div>
                        </div>
                        <DialogFooter>
                            <Button
                                type="button"
                                variant="outline"
                                onClick={() => setIsUploadOpen(false)}
                            >
                                {t('cancel')}
                            </Button>
                            <Button type="submit">
                                <Upload className="mr-2 h-3.5 w-3.5" />
                                {t('upload')}
                            </Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>
        </div>
    )
}
