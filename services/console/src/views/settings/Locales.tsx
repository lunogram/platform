import * as React from 'react'
import { useCallback, useContext, useState } from 'react'
import { useForm } from 'react-hook-form'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import type { Locale, LocaleOption, FieldProps } from '../../types'
import type { TextInputProps } from '../../ui/form/TextInput'
import type { FieldPath, FieldValues } from 'react-hook-form'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuGroup,
    DropdownMenuItem,
    DropdownMenuLabel,
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
    flexRender,
    getCoreRowModel,
    getFilteredRowModel,
    getSortedRowModel,
    useReactTable,
    type ColumnDef,
    type ColumnFiltersState,
    type SortingState,
    type VisibilityState,
} from '@tanstack/react-table'
import { MoreHorizontal } from 'lucide-react'
import { FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage, Form } from '@/components/ui/form'
import { PlusIcon } from '../../components/icons'
import { languageName } from '../../utils'
import { useTranslation } from 'react-i18next'

type LocaleFieldProps<X extends FieldValues, P extends FieldPath<X>> = TextInputProps<string> & 
    FieldProps<X, P>

export const LocaleTextField = <X extends FieldValues, P extends FieldPath<X>>(params: LocaleFieldProps<X, P>) => {
    const { t } = useTranslation()
    const {
        form,
        name,
        required,
        subtitle,
        disabled,
        readOnly,
        onFocus,
        onChange,
        type = 'text',
    } = params

    const initialLocale = form.getValues(name) as string | undefined
    const [language, setLanguage] = useState<string | undefined>(initialLocale ? languageName(initialLocale) : undefined)

    const handlePreviewLanguage = (locale: string) => {
        if (!locale) {
            setLanguage(undefined)
            return
        }
        setLanguage(languageName(locale))
    }

    return (
        <Form {...form}>
            <FormField
                control={form.control}
                name={name}
                rules={{
                    required
                }}
                render={({ field }) => (
                    <FormItem>
                        <FormLabel className="inline-flex gap-1">
                            <span>
                                {params.label ?? t('locale.singular')}
                                {required && <span className="text-destructive">*</span>}
                            </span>
                        </FormLabel>
                        {subtitle && <FormDescription>{subtitle}</FormDescription>}
                        <FormControl>
                            <div className="flex items-center gap-2">
                                <Input
                                    {...field}
                                    type={type}
                                    disabled={disabled}
                                    readOnly={readOnly}
                                    value={field.value ?? ''}
                                    onFocus={onFocus}
                                    onChange={event => {
                                        field.onChange(event)
                                        handlePreviewLanguage(event.target.value)
                                        onChange?.(event.target.value)
                                    }}
                                />
                                {language && (
                                    <Badge variant="secondary" className="whitespace-nowrap">
                                        {language}
                                    </Badge>
                                )}
                            </div>
                        </FormControl>
                    <FormMessage />
                </FormItem>
                )}
            />
        </Form>
    )
}

export default function Locales() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [open, setOpen] = useState(false)
    const [locales, setLocales] = useState<Locale[]>([])
    const [loading, setLoading] = useState(true)
    const [sorting, setSorting] = React.useState<SortingState>([])
    const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
    const [columnVisibility, setColumnVisibility] = React.useState<VisibilityState>({})
    const [rowSelection, setRowSelection] = React.useState({})
    const [localePreview, setLocalePreview] = useState<string | undefined>()

    const form = useForm<Pick<LocaleOption, 'key'>>({
        defaultValues: {
            key: ''
        }
    })

    const loadLocales = useCallback(async () => {
        setLoading(true)
        try {
            const response = await api.locales.search(project.id, { limit: 100 })
            setLocales(response.results)
        } finally {
            setLoading(false)
        }
    }, [project.id])

    React.useEffect(() => {
        loadLocales()
    }, [loadLocales])

    const handleDeleteLocale = async (locale: Locale) => {
        if (!confirm(t('locale.delete_confirmation'))) return
        await api.locales.delete(project.id, locale.id)
        await loadLocales()
    }

    const columns: ColumnDef<Locale>[] = [
        {
            id: "key",
            accessorKey: "key",
            header: t('key'),
            cell: ({ row }) => <div>{row.getValue("key")}</div>,
        },
        {
            id: "label",
            accessorKey: "label",
            header: t('label'),
            cell: ({ row }) => <div>{row.getValue("label")}</div>,
        },
        {
            id: "actions",
            accessorKey: "actions",
            header: t('action'),
            cell: ({ row }) => {
                const locale = row.original
                return (
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
                                <DropdownMenuItem
                                    className="text-destructive"
                                    onClick={async () => await handleDeleteLocale(locale)}
                                >
                                    {t('delete')}
                                </DropdownMenuItem>
                            </DropdownMenuGroup>
                        </DropdownMenuContent>
                    </DropdownMenu>
                )
            },
        },
    ]

    const table = useReactTable({
        data: locales,
        columns,
        onSortingChange: setSorting,
        onColumnFiltersChange: setColumnFilters,
        getCoreRowModel: getCoreRowModel(),
        getSortedRowModel: getSortedRowModel(),
        getFilteredRowModel: getFilteredRowModel(),
        onColumnVisibilityChange: setColumnVisibility,
        onRowSelectionChange: setRowSelection,
        state: {
            sorting,
            columnFilters,
            columnVisibility,
            rowSelection,
        },
    })

    return (
        <>
            <div className="w-full">
                <div className="flex items-center py-4">
                    <h2 className="text-2xl">{t('locales')}</h2>
                    <Button
                        size="sm"
                        className="ml-auto"
                        onClick={() => setOpen(true)}
                    >
                        <PlusIcon />
                        {t('create_locale')}
                    </Button>
                </div>
                <div className="overflow-hidden rounded-md border">
                    <Table>
                        <TableHeader>
                            {table.getHeaderGroups().map((headerGroup) => (
                                <TableRow key={headerGroup.id}>
                                    {headerGroup.headers.map((header) => {
                                        return (
                                            <TableHead key={header.id}>
                                                {header.isPlaceholder
                                                    ? null
                                                    : flexRender(
                                                        header.column.columnDef.header,
                                                        header.getContext()
                                                    )}
                                            </TableHead>
                                        )
                                    })}
                                </TableRow>
                            ))}
                        </TableHeader>
                        <TableBody>
                            {loading ? (
                                <TableRow>
                                    <TableCell
                                        colSpan={columns.length}
                                        className="h-24 text-center"
                                    >
                                        {t('loading')}
                                    </TableCell>
                                </TableRow>
                            ) : table.getRowModel().rows?.length ? (
                                table.getRowModel().rows.map((row) => (
                                    <TableRow
                                        key={row.id}
                                        data-state={row.getIsSelected() && "selected"}
                                    >
                                        {row.getVisibleCells().map((cell) => (
                                            <TableCell key={cell.id}>
                                                {flexRender(
                                                    cell.column.columnDef.cell,
                                                    cell.getContext()
                                                )}
                                            </TableCell>
                                        ))}
                                    </TableRow>
                                ))
                            ) : (
                                <TableRow>
                                    <TableCell
                                        colSpan={columns.length}
                                        className="h-24 text-center"
                                    >
                                    </TableCell>
                                </TableRow>
                            )}
                        </TableBody>
                    </Table>
                </div>
            </div>
            <Dialog open={open} onOpenChange={setOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('create_locale')}</DialogTitle>
                    </DialogHeader>
                    <Form {...form}>
                        <form onSubmit={form.handleSubmit(async ({ key }) => {
                            await api.locales.create(project.id, { key, label: languageName(key) ?? key })
                            await loadLocales()
                            setOpen(false)
                            form.reset({ key: ''})
                        })}>
                            <FormField
                                control={form.control}
                                name="key"
                                rules={{ required: true }}
                                render={({ field }) => (
                                    <FormItem>
                                        <FormLabel className="inline-flex gap-1">
                                            {t('locale.singular')}
                                            <span className="text-destructive">*</span>
                                        </FormLabel>
                                        <FormDescription>
                                            {t('locale.field_subtitle')}
                                        </FormDescription>
                                        <div className="flex items-center gap-2">
                                            <FormControl>
                                                <Input 
                                                    {...field} 
                                                    type="text"
                                                    onChange={(e) => {
                                                        field.onChange(e)
                                                        const value = e.target.value
                                                        setLocalePreview(value ? languageName(value) : undefined)
                                                    }}
                                                />
                                            </FormControl>
                                            {localePreview && (
                                                <Badge variant="secondary" className="whitespace-nowrap">
                                                    {localePreview}
                                                </Badge>
                                            )}
                                        </div>
                                        <FormMessage />
                                    </FormItem>
                                )}
                            />
                            <Button type="submit" className="mt-4">
                                {t('create')}
                            </Button>
                        </form>
                    </Form>
                </DialogContent>
            </Dialog>
        </>
    )
}
