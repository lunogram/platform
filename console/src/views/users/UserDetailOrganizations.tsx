import React, { useCallback, useContext, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Building2,
    ChevronDown,
    ChevronRight,
    ChevronLeft,
    ExternalLink,
    Search,
} from 'lucide-react'
import { ProjectContext, UserContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'
import { useResolver } from '../../hooks'
import { useRoute } from '../router'
import { formatDate, cn } from '../../utils'
import { getRandomColor } from '@/lib/colors'
import oapiClient, { type Organization } from '../../oapi/client'
import type { SearchParams, SearchResult } from '../../types'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { JsonView } from '@/components/ui/json-view'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'

function getPageNumbers(current: number, total: number): (number | '...')[] {
    if (total <= 7) {
        return Array.from({ length: total }, (_, i) => i + 1)
    }
    if (current <= 3) {
        return [1, 2, 3, 4, 5, '...', total]
    }
    if (current >= total - 2) {
        return [1, '...', total - 4, total - 3, total - 2, total - 1, total]
    }
    return [1, '...', current - 1, current, current + 1, '...', total]
}

interface OrgExpandedRowProps {
    organization: Organization
}

function OrgExpandedRow({ organization }: OrgExpandedRowProps) {
    const { t } = useTranslation()
    const route = useRoute()
    const hasData = organization.data && Object.keys(organization.data as Record<string, unknown>).length > 0

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={4} className="p-0">
                <div className="px-6 py-4 space-y-4">
                    {/* Organization Data */}
                    {hasData && (
                        <div>
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">
                                {t('organization_data', 'Organization data')}
                            </p>
                            <JsonView
                                data={organization.data as Record<string, unknown>}
                                defaultExpanded
                            />
                        </div>
                    )}

                    {!hasData && (
                        <p className="text-sm text-muted-foreground">
                            {t('no_organization_data', 'No additional data')}
                        </p>
                    )}

                    {/* View Link */}
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={(e) => {
                            e.stopPropagation()
                            route(`organizations/${organization.id}`)
                        }}
                    >
                        <ExternalLink className="mr-2 h-4 w-4" />
                        {t('view_organization', 'View organization')}
                    </Button>
                </div>
            </TableCell>
        </TableRow>
    )
}

export default function UserDetailOrganizations() {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)

    const [page, setPage] = useState(1)
    const [searchQuery, setSearchQuery] = useState('')
    const [debouncedQuery, setDebouncedQuery] = useState('')
    const [expandedOrgId, setExpandedOrgId] = useState<string | null>(null)
    const searchTimeoutRef = React.useRef<ReturnType<typeof setTimeout>>()
    const limit = 25

    const handleSearch = (value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }

    const [result] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET('/api/admin/projects/{projectID}/subjects/users/{userID}/subject-organizations', {
                params: {
                    path: {
                        projectID: project.id,
                        userID: user.id,
                    },
                    query: {
                        limit,
                        offset: (page - 1) * limit,
                        search: debouncedQuery || undefined,
                    },
                },
            })
            if (!data) return null
            return {
                results: data.results,
                total: data.total ?? 0,
            }
        }, [project.id, user.id, page, debouncedQuery])
    )

    const organizations = result?.results
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const getOrgDisplayName = (org: Organization) => {
        if (org.name) return org.name
        return org.external_id ?? org.id
    }

    const getOrgInitials = (org: Organization) => {
        const name = getOrgDisplayName(org)
        return name.substring(0, 2).toUpperCase()
    }

    const toggleExpand = (orgId: string) => {
        setExpandedOrgId(expandedOrgId === orgId ? null : orgId)
    }

    return (
        <div className="space-y-4">
            {/* Search */}
            <div className="flex items-center gap-4">
                <div className="relative max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t('search_organizations', 'Search organizations...')}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
            </div>

            {/* Organizations Table */}
            <div className="border rounded-lg">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-8 p-0"></TableHead>
                            <TableHead>{t('name')}</TableHead>
                            <TableHead>{t('external_id')}</TableHead>
                            <TableHead>{t('created_at')}</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {organizations === undefined ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell className="p-0 pl-2"><Skeleton className="h-4 w-4" /></TableCell>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-lg" />
                                            <Skeleton className="h-4 w-36" />
                                        </div>
                                    </TableCell>
                                    <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                    <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                                </TableRow>
                            ))
                        ) : organizations.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <Building2 className="h-6 w-6 text-muted-foreground" />
                                        </div>
                                        <p className="font-medium mb-1">
                                            {debouncedQuery ? t('no_organizations_found', 'No organizations found') : t('no_organizations_yet', 'No organizations')}
                                        </p>
                                        <p className="text-sm text-muted-foreground max-w-xs text-center">
                                            {t('no_user_organizations_description', 'Organizations this user belongs to will appear here')}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            organizations.map((org) => {
                                const isExpanded = expandedOrgId === org.id
                                const orgColor = getRandomColor(org.external_id ?? org.id)

                                return (
                                    <React.Fragment key={org.id}>
                                        <TableRow
                                            className={cn(
                                                'cursor-pointer',
                                                isExpanded && 'bg-muted/50',
                                            )}
                                            onClick={() => toggleExpand(org.id)}
                                        >
                                            <TableCell className="p-0 pl-3">
                                                {isExpanded
                                                    ? <ChevronDown className="h-4 w-4 text-muted-foreground" />
                                                    : <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                                }
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div
                                                        className="flex h-8 w-8 items-center justify-center rounded-lg text-white text-xs font-medium shrink-0"
                                                        style={{ backgroundColor: orgColor }}
                                                    >
                                                        {getOrgInitials(org)}
                                                    </div>
                                                    <span className="font-medium">
                                                        {getOrgDisplayName(org)}
                                                    </span>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                {org.external_id ? (
                                                    <code className="text-sm text-muted-foreground">{org.external_id}</code>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                {formatDate(preferences, org.created_at, 'PP')}
                                            </TableCell>
                                        </TableRow>

                                        {/* Expanded Row */}
                                        {isExpanded && (
                                            <OrgExpandedRow key={`${org.id}-expanded`} organization={org} />
                                        )}
                                    </React.Fragment>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {total > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {total === 1 ? t('organization') : t('organizations')}
                        </p>
                        {totalPages > 1 && (
                            <div className="flex items-center gap-1">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => setPage(p => p - 1)}
                                    disabled={!hasPrevPage}
                                    className="h-8 w-8 p-0"
                                >
                                    <ChevronLeft className="h-4 w-4" />
                                </Button>

                                {getPageNumbers(page, totalPages).map((pageNum, idx) => (
                                    pageNum === '...' ? (
                                        <span key={`ellipsis-${idx}`} className="px-1 text-muted-foreground">
                                            ...
                                        </span>
                                    ) : (
                                        <Button
                                            key={pageNum}
                                            variant={page === pageNum ? 'default' : 'ghost'}
                                            size="sm"
                                            onClick={() => setPage(pageNum as number)}
                                            className="h-8 w-8 p-0"
                                        >
                                            {pageNum}
                                        </Button>
                                    )
                                ))}

                                <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => setPage(p => p + 1)}
                                    disabled={!hasNextPage}
                                    className="h-8 w-8 p-0"
                                >
                                    <ChevronRight className="h-4 w-4" />
                                </Button>
                            </div>
                        )}
                    </div>
                )}
            </div>
        </div>
    )
}
