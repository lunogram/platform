import type { Key, ReactNode } from "react"
import { useState, useCallback, useMemo, useContext } from "react"
import { useSearchParams } from "react-router"
import { useDebounceControl, useResolver } from "@/hooks"
import type { SearchParams, SearchResult } from "@/types"
import { cn, formatDate, prune, snakeToTitle } from "@/utils"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import CursorPagination from "@/components/pagination"
import { CheckIcon, ChevronDownIcon, ChevronUpDownIcon, ChevronUpIcon } from "@/components/icons"
import { CloseIcon } from "@/components/icons"
import { Search } from "lucide-react"
import { useTranslation } from "react-i18next"
import { PreferencesContext } from "@/contexts/PreferencesContext"

type DataTableResolver<T, R> = (args: { item: T }) => R

export interface DataTableCol<T> {
    key: string
    title?: ReactNode
    cell?: DataTableResolver<T, ReactNode>
    sortable?: boolean
    sortKey?: string
    minWidth?: number | string
}

export interface ColSort {
    sort: string
    direction: string
}

export interface SearchTableProps<T extends Record<string, any>> {
    columns: Array<DataTableCol<T>>
    title?: ReactNode
    description?: ReactNode
    actions?: ReactNode
    results: SearchResult<T> | null
    filters?: ReactNode[]
    params: SearchParams
    setParams: (params: SearchParams) => void
    enableSearch?: boolean
    searchPlaceholder?: string
    tagEntity?: "journeys" | "lists" | "users" | "campaigns"
    itemKey?: DataTableResolver<T, Key>
    emptyMessage?: ReactNode
    selectedRow?: Key
    onSelectRow?: (row: T) => void
}

const DEFAULT_ITEMS_PER_PAGE = 25
const DEFAULT_PAGE = 0

const toTableParams = (searchParams: URLSearchParams): SearchParams => {
    return {
        cursor: searchParams.get("cursor") ?? undefined,
        page: searchParams.get("page") === "prev" ? "prev" : "next",
        limit: parseInt(searchParams.get("limit") ?? "25"),
        search: searchParams.get("search") ?? undefined,
        tag: searchParams.getAll("tag"),
        sort: searchParams.get("sort") ?? undefined,
        direction: searchParams.get("direction") ?? undefined,
        filter: searchParams.get("filter") ? JSON.parse(searchParams.get("filter")!) : undefined,
    }
}

const fromTableParams = (params: Partial<SearchParams>): Record<string, string> => {
    return prune({
        cursor: params.cursor,
        page: params.page,
        limit: (params.limit ?? DEFAULT_ITEMS_PER_PAGE).toString(),
        search: params.search,
        tag: params.tag ?? [],
        sort: params.sort,
        direction: params.direction,
        filter: params.filter ? JSON.stringify(params.filter) : undefined,
    })
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTableSearchParams = (initialParams: Partial<SearchParams> = {}) => {
    const [searchParams, setSearchParams] = useSearchParams({
        page: DEFAULT_PAGE.toString(),
        ...fromTableParams(initialParams),
    })

    const setParams = useCallback<
        (params: SearchParams | ((prev: SearchParams) => SearchParams)) => void
    >(
        (next) => {
            if (typeof next === "function") {
                setSearchParams((prev) => fromTableParams(next(toTableParams(prev))))
            } else {
                setSearchParams(fromTableParams(next))
            }
        },
        [setSearchParams],
    )

    const str = searchParams.toString()

    return useMemo(
        () => [toTableParams(new URLSearchParams(str)), setParams] as const,
        [str, setParams],
    )
}

/**
 * local state
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useSearchTableState<T>(
    loader: (params: SearchParams) => Promise<SearchResult<T> | null>,
    initialParams?: Partial<SearchParams>,
) {
    const [params, setParams] = useState<SearchParams>({
        limit: 25,
        search: "",
        ...(initialParams ?? {}),
    })

    const [results, , reload] = useResolver(
        useCallback(async () => await loader(params), [loader, params]),
    )

    return {
        params,
        reload,
        results,
        setParams,
    }
}

export interface SearchTableQueryState<T> {
    results: SearchResult<T> | null
    params: SearchParams
    reload: () => Promise<void>
    setParams: (params: SearchParams) => void
}

/**
 * global query string state
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useSearchTableQueryState<T>(
    loader: (params: SearchParams) => Promise<SearchResult<T> | null>,
    initialParams?: Partial<SearchParams>,
): SearchTableQueryState<T> {
    const [params, setParams] = useTableSearchParams(initialParams)

    const [results, , reload] = useResolver(
        useCallback(async () => await loader(params), [loader, params]),
    )

    return {
        params,
        reload,
        results,
        setParams,
    }
}

function SortableHeaderCell<T>({
    col,
    columnSort,
    onColumnSort,
}: {
    col: DataTableCol<T>
    columnSort?: ColSort
    onColumnSort?: (sort?: ColSort) => void
}) {
    const { key, title, sortable, sortKey } = col
    const sort = sortKey ?? key

    const handleSort = () => {
        if (columnSort?.sort !== sort) {
            onColumnSort?.({ sort, direction: "asc" })
        } else if (columnSort?.direction === "desc") {
            onColumnSort?.()
        } else {
            onColumnSort?.({ sort, direction: "desc" })
        }
    }

    return (
        <TableHead>
            <div className="flex items-center gap-1">
                <span>{title ?? snakeToTitle(key)}</span>
                {sortable && (
                    <Button size="icon" variant="ghost" className="h-6 w-6" onClick={handleSort}>
                        {columnSort?.sort === sort ? (
                            columnSort?.direction === "asc" ? (
                                <ChevronUpIcon />
                            ) : (
                                <ChevronDownIcon />
                            )
                        ) : (
                            <ChevronUpDownIcon />
                        )}
                    </Button>
                )}
            </div>
        </TableHead>
    )
}

export function SearchTable<T extends Record<string, any>>({
    actions,
    columns,
    description,
    emptyMessage = "No Results",
    enableSearch,
    filters: additionalFilters = [],
    itemKey,
    onSelectRow,
    params,
    results,
    searchPlaceholder,
    selectedRow,
    setParams,
    tagEntity: _tagEntity,
    title,
}: SearchTableProps<T>) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)

    const [search, setSearch] = useDebounceControl(params.search ?? "", (search) =>
        setParams({ ...params, search }),
    )

    const columnSort = params.sort
        ? { sort: params.sort, direction: params.direction ?? "asc" }
        : undefined

    const handleColumnSort = (onSort?: ColSort) => {
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        const { sort: _sort, direction: _direction, ...prevParams } = params
        setParams({ ...prevParams, ...onSort })
    }

    const isLoading = !results
    const items = results?.results

    // Build the filter bar (search + any extra filters)
    const filters: ReactNode[] = []
    if (enableSearch) {
        filters.push(
            <div key="search" className="relative w-60">
                <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                    value={search}
                    placeholder={searchPlaceholder ?? t("search")}
                    onChange={(e) => setSearch(e.target.value)}
                    className="pl-8"
                />
            </div>,
        )
    }
    if (additionalFilters) {
        filters.push(...additionalFilters)
    }

    return (
        <>
            {/* Title / description / actions */}
            {(title || actions || description) && (
                <div className="flex items-center justify-between flex-wrap gap-2.5 my-5 mb-2.5">
                    <div className="flex flex-col">
                        <h3 className="m-0 shrink-0 font-semibold tracking-tight">{title}</h3>
                        {description && <div className="mt-1">{description}</div>}
                    </div>
                    {actions && (
                        <div className="flex justify-end self-start gap-2.5">{actions}</div>
                    )}
                </div>
            )}

            {/* Filters row */}
            {filters.length > 0 && <div className="flex gap-2.5 mb-2.5 flex-wrap">{filters}</div>}

            {/* Table */}
            <div className="rounded-md border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            {columns.map((col) => (
                                <SortableHeaderCell<T>
                                    key={col.key}
                                    col={col}
                                    columnSort={columnSort}
                                    onColumnSort={handleColumnSort}
                                />
                            ))}
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {items && items.length > 0 ? (
                            items.map((item) => {
                                const args = { item }
                                const key = itemKey ? itemKey(args) : (item as any).id

                                return (
                                    <TableRow
                                        key={key}
                                        className={cn(
                                            onSelectRow && "cursor-pointer",
                                            selectedRow === key && "bg-muted",
                                        )}
                                        onClick={() => onSelectRow?.(item)}
                                        data-state={selectedRow === key ? "selected" : undefined}
                                    >
                                        {columns.map((col) => {
                                            let value: any = col.cell
                                                ? col.cell(args)
                                                : item[col.key as keyof T]
                                            if (!col.cell) {
                                                if (
                                                    (col.key.endsWith("_at") ||
                                                        col.key.endsWith("_until")) &&
                                                    (typeof value === "string" ||
                                                        typeof value === "number")
                                                ) {
                                                    value = formatDate(preferences, value, "Pp")
                                                }
                                                if (typeof value === "boolean") {
                                                    value = value ? <CheckIcon /> : <CloseIcon />
                                                }
                                            }
                                            return (
                                                <TableCell
                                                    key={col.key}
                                                    style={
                                                        col.minWidth
                                                            ? { minWidth: col.minWidth }
                                                            : undefined
                                                    }
                                                >
                                                    {value ?? <>&#8211;</>}
                                                </TableCell>
                                            )
                                        })}
                                    </TableRow>
                                )
                            })
                        ) : isLoading ? (
                            Array.from({ length: 3 }, (_, i) => (
                                <TableRow key={i}>
                                    {columns.map((col) => (
                                        <TableCell key={col.key}>
                                            <Skeleton className="h-4 w-full" />
                                        </TableCell>
                                    ))}
                                </TableRow>
                            ))
                        ) : (
                            <TableRow>
                                <TableCell colSpan={columns.length} className="text-center">
                                    {emptyMessage}
                                </TableCell>
                            </TableRow>
                        )}
                    </TableBody>
                </Table>
            </div>

            {/* Pagination */}
            {results && (
                <div>
                    <CursorPagination
                        nextCursor={results.nextCursor}
                        prevCursor={results.prevCursor}
                        onPrev={(cursor) => setParams({ ...params, cursor, page: "prev" })}
                        onNext={(cursor) => setParams({ ...params, cursor, page: "next" })}
                    />
                </div>
            )}
        </>
    )
}
