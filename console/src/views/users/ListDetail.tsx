import { useCallback, useContext, useEffect, useState } from 'react'
import api from '../../api'
import { ListContext, ProjectContext } from '../../contexts'
import type { DynamicList, ListUpdateParams, Rule, WrapperRule } from '../../types'
import { Button } from '@/components/ui/button'
import RuleBuilder from './rules/RuleBuilder'
import { snakeToTitle } from '../../utils'
import { useSearchTableState } from '../../ui/SearchTable'
import { useRoute } from '../router'
import { ArchiveIcon, EditIcon, RestartIcon, SendIcon, UploadIcon } from '../../components/icons'
import { useTranslation } from 'react-i18next'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuGroup,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MoreHorizontal, Users } from 'lucide-react'
import { useBlocker } from 'react-router'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationNext,
    PaginationPrevious,
} from '@/components/ui/pagination'
import { Separator } from '@/components/ui/separator'
import { Field } from '@/components/ui/field'
import { Skeleton } from '@/components/ui/skeleton'

interface RuleSectionProps {
    list: DynamicList
    isSaving: boolean
    onRuleSave: (rule: Rule) => void
    onChange?: (rule: Rule) => void
}

const RuleSection = ({ list, isSaving, onRuleSave, onChange }: RuleSectionProps) => {
    const { t } = useTranslation()
    const [rule, setRule] = useState<Rule>(list.rule)
    const onSetRule = (rule: Rule) => {
        setRule(rule)
        onChange?.(rule)
    }
    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <h3 className="text-lg font-semibold">{t('rules')}</h3>
                <Button
                    size="sm"
                    onClick={() => onRuleSave(rule)}
                    disabled={isSaving}
                >
                    {isSaving ? t('saving') : t('rules_save')}
                </Button>
            </div>
            <RuleBuilder rule={rule} setRule={onSetRule} />
        </div>
    )
}



export default function ListDetail() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [list, setList] = useContext(ListContext)
    const [isEditListOpen, setIsEditListOpen] = useState(false)
    const [isUploadOpen, setIsUploadOpen] = useState(false)
    const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)
    const [isSaving, setIsSaving] = useState(false)
    const [error, setError] = useState<string | undefined>()
    const [editName, setEditName] = useState(list.name)
    const [uploadFile, setUploadFile] = useState<File | undefined>(undefined)

    const state = useSearchTableState(useCallback(async params => await api.lists.users(project.id, list.id, params), [list, project]))
    const route = useRoute()

    const refreshList = useCallback(() => {
        api.lists.get(project.id, list.id)
            .then(setList)
            .then(() => state.reload)
            .catch(() => { })
    }, [project.id, list.id, setList, state.reload])

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

    const handleEditSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        await saveList({ name: editName })
    }

    const uploadUsers = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!uploadFile) return
        await api.lists.upload(project.id, list.id, uploadFile)
        refreshList()
        setIsUploadOpen(false)
        setUploadFile(undefined)
    }

    const handleRecountList = async () => {
        await api.lists.recount(project.id, list.id)
        window.location.reload()
    }

    const handleArchiveList = async () => {
        await api.lists.delete(project.id, list.id)
        window.location.href = `/projects/${project.id}/lists`
    }

    return (
        <div className="container mx-auto p-6 space-y-6">
            <div className="space-y-4">
                <div className="flex items-start justify-between">
                    <div className="space-y-1">
                        <h1 className="text-2xl font-bold tracking-tight">{list.name}</h1>
                        <Table>
                            <TableBody>
                                <TableRow className="border-0 hover:bg-transparent">
                                    <TableCell className="py-1 pl-0 text-muted-foreground">{t('type')}</TableCell>
                                    <TableCell className="py-1">{snakeToTitle(list.type)}</TableCell>
                                </TableRow>
                                <TableRow className="border-0 hover:bg-transparent">
                                    <TableCell className="py-1 pl-0 text-muted-foreground">{t('users_count')}</TableCell>
                                    <TableCell className="py-1">{list.state === 'loading' ? '–' : list.users_count?.toLocaleString()}</TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>
                    <div className="flex items-center gap-2">
                        {list.state === 'draft' && (
                            <Button
                                onClick={async () => await saveList({ name: list.name, published: true })}
                            >
                                <SendIcon />
                                {t('publish')}
                            </Button>
                        )}
                        {list.type === 'static' && (
                            <Button
                                variant="secondary"
                                onClick={() => setIsUploadOpen(true)}
                            >
                                <UploadIcon />
                                {t('upload_list')}
                            </Button>
                        )}
                        <Button onClick={() => {
                            setEditName(list.name)
                            setIsEditListOpen(true)
                        }}>
                            <EditIcon />
                            {t('edit_list')}
                        </Button>
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button
                                    variant="ghost"
                                    className="h-8 w-8 p-0"
                                    aria-label={t('action')}
                                >
                                    <MoreHorizontal className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuGroup>
                                    <DropdownMenuLabel>{t('action')}</DropdownMenuLabel>
                                    <DropdownMenuItem onClick={async () => await handleRecountList()}>
                                        <RestartIcon />
                                        {t('recount')}
                                    </DropdownMenuItem>
                                    <DropdownMenuItem
                                        className="text-destructive"
                                        onClick={async () => await handleArchiveList()}
                                    >
                                        <ArchiveIcon />
                                        {t('archive')}
                                    </DropdownMenuItem>
                                </DropdownMenuGroup>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </div>
                </div>
            </div>

            <Separator />

            {error && (
                <Alert variant="destructive">
                    <AlertTitle>Error</AlertTitle>
                    <AlertDescription>{error}</AlertDescription>
                </Alert>
            )}

            {list.type === 'dynamic' && (
                <>
                    <RuleSection
                        list={list}
                        isSaving={isSaving}
                        onRuleSave={async (rule) => await saveList({ name: list.name, rule: rule as WrapperRule })}
                        onChange={() => setHasUnsavedChanges(true)}
                    />
                    <Separator />
                </>
            )}

            <div className="space-y-4">
                <h3 className="text-lg font-semibold">{t('users')}</h3>
                <div className="rounded-md border">
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
                            {state.results === null ? (
                                Array.from({ length: 5 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                    </TableRow>
                                ))
                            ) : (
                                state.results.results.map((user) => (
                                    <TableRow
                                        key={user.id}
                                        className="cursor-pointer"
                                        onClick={() => route(`users/${user.id}`)}
                                    >
                                        <TableCell className="font-medium">{user.full_name ?? '–'}</TableCell>
                                        <TableCell>{user.external_id ?? '–'}</TableCell>
                                        <TableCell>{user.email ?? '–'}</TableCell>
                                        <TableCell>{user.phone ?? '–'}</TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>
                {state.results && state.results.results.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-12 text-center">
                        <div className="rounded-full bg-muted p-4 mb-4">
                            <Users className="h-8 w-8 text-muted-foreground" />
                        </div>
                        <h3 className="text-lg font-semibold mb-2">{t('no_users_title')}</h3>
                        <p className="text-sm text-muted-foreground max-w-sm">
                            {t('no_users_description')}
                        </p>
                    </div>
                )}
                {state.results && state.results.results.length > 0 && (
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

            <Dialog open={isEditListOpen} onOpenChange={setIsEditListOpen}>
                <DialogContent className="max-w-lg">
                    <DialogHeader>
                        <DialogTitle>{t('edit_list')}</DialogTitle>
                    </DialogHeader>
                    <form onSubmit={handleEditSubmit} className="space-y-4">
                        <Field>
                            <label className="text-sm font-medium">
                                {t('list_name')}
                                <span className="text-destructive">*</span>
                            </label>
                            <Input
                                value={editName}
                                onChange={(e) => setEditName(e.target.value)}
                                required
                            />
                        </Field>
                        <div className="flex justify-end gap-2">
                            <Button type="button" variant="ghost" onClick={() => setIsEditListOpen(false)}>
                                {t('cancel')}
                            </Button>
                            <Button type="submit" disabled={isSaving}>
                                {isSaving ? t('saving') : t('save')}
                            </Button>
                        </div>
                    </form>
                </DialogContent>
            </Dialog>

            <Dialog open={isUploadOpen} onOpenChange={setIsUploadOpen}>
                <DialogContent className="max-w-lg">
                    <DialogHeader>
                        <DialogTitle>{t('import_users')}</DialogTitle>
                        <DialogDescription>{t('upload_instructions')}</DialogDescription>
                    </DialogHeader>
                    <form onSubmit={uploadUsers} className="space-y-4">
                        <Field>
                            <label className="text-sm font-medium">
                                {t('file')}
                                <span className="text-destructive">*</span>
                            </label>
                            <Input
                                type="file"
                                onChange={(e) => setUploadFile(e.target.files?.[0] || undefined)}
                                required
                            />
                        </Field>
                        <div className="flex justify-end gap-2">
                            <Button type="button" variant="ghost" onClick={() => setIsUploadOpen(false)}>
                                {t('cancel')}
                            </Button>
                            <Button type="submit" disabled={!uploadFile}>
                                {t('upload')}
                            </Button>
                        </div>
                    </form>
                </DialogContent>
            </Dialog>
        </div>
    )
}
