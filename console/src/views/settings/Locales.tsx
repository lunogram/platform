import { useCallback, useContext, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Globe, Plus, Search, MoreHorizontal } from 'lucide-react'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import { useResolver } from '../../hooks'
import type { Locale } from '../../types'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { LocalePicker } from '@/components/locale/picker'
import { resolveLocaleName } from '@/components/locale/locales'

export default function Locales() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [open, setOpen] = useState(false)
    const [searchQuery, setSearchQuery] = useState('')
    const [selectedLocaleKey, setSelectedLocaleKey] = useState<string | undefined>()
    const [isCreating, setIsCreating] = useState(false)

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.locales.search(project.id, { limit: 100 })
        }, [project.id])
    )

    const locales = result?.results ?? []
    const filteredLocales = searchQuery
        ? locales.filter(l =>
            l.key.toLowerCase().includes(searchQuery.toLowerCase()) ||
            l.label.toLowerCase().includes(searchQuery.toLowerCase())
        )
        : locales

    // Keys already added to the project — exclude from picker
    const existingKeys = locales.map(l => l.key)

    const handleDeleteLocale = async (locale: Locale) => {
        if (!confirm(t('locale.delete_confirmation'))) return
        await api.locales.delete(project.id, locale.id)
        await reload()
    }

    const handleCreate = async () => {
        if (!selectedLocaleKey) return
        setIsCreating(true)
        try {
            const label = resolveLocaleName(selectedLocaleKey)
            await api.locales.create(project.id, { key: selectedLocaleKey, label })
            await reload()
            setOpen(false)
            setSelectedLocaleKey(undefined)
        } finally {
            setIsCreating(false)
        }
    }

    return (
        <div className="flex flex-col gap-6">
            {/* Header */}
            <h2 className="text-2xl font-semibold tracking-tight">{t('locales')}</h2>

            {/* Search and Actions */}
            <div className="flex items-center justify-between gap-4">
                <div className="relative max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t('search')}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <Button onClick={() => setOpen(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    {t('create_locale')}
                </Button>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t('key')}</TableHead>
                            <TableHead>{t('label')}</TableHead>
                            <TableHead className="w-[70px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {result === null ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                                </TableRow>
                            ))
                        ) : filteredLocales.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={3} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Globe className="h-8 w-8" />
                                        <p>{searchQuery ? t('no_results') : t('no_locales_yet', 'No locales yet')}</p>
                                        {!searchQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setOpen(true)}
                                                className="mt-2"
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t('create_locale')}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            filteredLocales.map((locale) => (
                                <TableRow key={locale.id}>
                                    <TableCell>
                                        <code className="rounded bg-muted px-1.5 py-0.5 text-sm font-medium">
                                            {locale.key}
                                        </code>
                                    </TableCell>
                                    <TableCell className="text-muted-foreground">{locale.label}</TableCell>
                                    <TableCell>
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
                                                <DropdownMenuItem
                                                    className="text-destructive"
                                                    onClick={async () => await handleDeleteLocale(locale)}
                                                >
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

                {/* Footer count */}
                {filteredLocales.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {filteredLocales.length} {filteredLocales.length === 1 ? t('locale.singular') : t('locales')}
                        </p>
                    </div>
                )}
            </div>

            {/* Add Locale Dialog */}
            <Dialog open={open} onOpenChange={(isOpen) => {
                setOpen(isOpen)
                if (!isOpen) {
                    setSelectedLocaleKey(undefined)
                }
            }}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('create_locale')}</DialogTitle>
                        <DialogDescription>
                            {t('locale.add_description', 'Select a language to add to this project.')}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label>{t('locale.picker.label', 'Language')}</Label>
                            <LocalePicker
                                value={selectedLocaleKey}
                                onChange={setSelectedLocaleKey}
                                exclude={existingKeys}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setOpen(false)}
                            disabled={isCreating}
                        >
                            {t('cancel')}
                        </Button>
                        <Button
                            onClick={handleCreate}
                            disabled={!selectedLocaleKey || isCreating}
                        >
                            {isCreating ? t('creating', 'Creating...') : t('create')}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
