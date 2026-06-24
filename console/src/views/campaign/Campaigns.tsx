import { useCallback, useContext, useState, useRef } from "react"
import { Link, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    Megaphone,
    Mail,
    Smartphone,
    MessageSquareDot,
    Inbox,
    MoreHorizontal,
    Copy,
    Archive,
    ArchiveRestore,
} from "lucide-react"
import { oapiClient } from "@/oapi/client"
import { useResolver } from "../../hooks"
import { formatDate, snakeToTitle } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert"
import { CampaignsIcon } from "@/components/icons"

import { CreateCampaign } from "./CreateCampaign"

import type { CampaignDelivery, ChannelType } from "@/types"
import type { UUID } from "@/types/common"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    sms: Smartphone,
    push: MessageSquareDot,
    inbox: Inbox,
}

function formatDelivery(delivery: CampaignDelivery) {
    const sent = delivery?.sent ?? 0
    const total = delivery?.total ?? 0
    const ratio = total > 0 ? sent / total : 0
    return `${sent.toLocaleString()} (${ratio.toLocaleString(undefined, { style: "percent", minimumFractionDigits: 0 })})`
}

interface CampaignsProps {
    create?: boolean
}

export default function Campaigns({ create = false }: CampaignsProps) {
    const pageSize = 25
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [offset, setOffset] = useState(0)
    const [archivedOffset, setArchivedOffset] = useState(0)
    const [showArchived, setShowArchived] = useState(false)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setOffset(0)
        setArchivedOffset(0)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const response = await oapiClient.GET("/api/admin/projects/{projectID}/campaigns", {
                params: {
                    path: {
                        projectID: project.id,
                    },
                    query: {
                        limit: pageSize,
                        offset,
                        search: debouncedQuery || undefined,
                    },
                },
            })

            if (response.error) {
                throw response.error
            }

            if (!response.data) {
                throw new Error("Failed to load campaigns")
            }

            return response.data
        }, [project.id, debouncedQuery, offset]),
    )

    const [archivedResult, , reloadArchived] = useResolver(
        useCallback(async () => {
            if (!showArchived) return null
            const response = await oapiClient.GET("/api/admin/projects/{projectID}/campaigns", {
                params: {
                    path: {
                        projectID: project.id,
                    },
                    query: {
                        limit: pageSize,
                        offset: archivedOffset,
                        include_deleted: true,
                        search: debouncedQuery || undefined,
                    },
                },
            })
            if (response.error || !response.data) return null
            return response.data
        }, [project.id, debouncedQuery, showArchived, archivedOffset]),
    )

    const isArchivedView = showArchived
    const campaigns = (isArchivedView ? archivedResult?.results : result?.results) ?? []
    const total = isArchivedView ? (archivedResult?.total ?? 0) : (result?.total ?? 0)
    const isLoading = isArchivedView ? archivedResult === null : !result
    const currentOffset = isArchivedView ? archivedOffset : offset
    const hasNextPage = currentOffset + pageSize < total
    const hasPrevPage = currentOffset > 0

    const handleNextPage = () => {
        if (!hasNextPage) return
        const setter = isArchivedView ? setArchivedOffset : setOffset
        setter((prev) => prev + pageSize)
    }

    const handlePrevPage = () => {
        if (!hasPrevPage) return
        const setter = isArchivedView ? setArchivedOffset : setOffset
        setter((prev) => Math.max(0, prev - pageSize))
    }

    const handleDuplicateCampaign = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        const response = await oapiClient.POST(
            "/api/admin/projects/{projectID}/campaigns/{campaignID}/duplicate",
            {
                params: {
                    path: {
                        projectID: project.id,
                        campaignID: id,
                    },
                },
            },
        )

        if (response.data?.id) {
            navigate(`/projects/${project.id}/campaigns/${response.data.id}`)
        }
    }

    const handleArchiveCampaign = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        await oapiClient.DELETE("/api/admin/projects/{projectID}/campaigns/{campaignID}", {
            params: {
                path: {
                    projectID: project.id,
                    campaignID: id,
                },
            },
        })
        setShowArchived(false)
        await reload()
    }

    const handleUnarchiveCampaign = async (id: string) => {
        const response = await oapiClient.POST(
            "/api/admin/projects/{projectID}/campaigns/{campaignID}/unarchive",
            {
                params: {
                    path: {
                        projectID: project.id,
                        campaignID: id,
                    },
                },
            },
        )
        if (response.error) {
            console.error("Failed to unarchive campaign", response.error)
            return
        }
        setShowArchived(false)
        setArchivedOffset(0)
        await Promise.all([reload(), reloadArchived()])
    }

    const handleRowClick = (campaign: { id: UUID } & unknown) => {
        navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}`)
    }

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <CampaignsIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">
                        {t("campaign.plural")}
                    </h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "campaigns_description",
                            "Create and manage email, SMS, and push notification campaigns to engage your audience.",
                        )}
                    </p>
                </div>
            </div>

            {/* Provider setup banner */}
            {project.has_provider === false && (
                <Alert>
                    <AlertTitle>{t("setup")}</AlertTitle>
                    <AlertDescription className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                        <span>{t("setup_integration_description")}</span>
                        <Link to={`/projects/${project.id}/integrations`} className="shrink-0">
                            <Button className="w-full sm:w-auto">{t("setup_integration")}</Button>
                        </Link>
                    </AlertDescription>
                </Alert>
            )}

            {/* Search and Actions */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_campaigns", "Search campaigns...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <div className="flex items-center gap-2">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 w-8 p-0"
                        aria-label={t("show_archived", "Show archived")}
                        aria-pressed={showArchived}
                        onClick={() => {
                            setArchivedOffset(0)
                            setShowArchived((prev) => !prev)
                        }}
                    >
                        <ArchiveRestore className="h-4 w-4" />
                    </Button>
                    <CreateCampaign open={create} />
                </div>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("delivery")}</TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("updated_at")}
                            </TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {isLoading ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-md" />
                                            <div className="space-y-1">
                                                <Skeleton className="h-4 w-36" />
                                                <Skeleton className="h-3 w-20" />
                                            </div>
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-24" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : campaigns.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Megaphone className="h-8 w-8" />
                                        <p>
                                            {isArchivedView
                                                ? t(
                                                      "no_archived_campaigns",
                                                      "No archived campaigns",
                                                  )
                                                : debouncedQuery
                                                  ? t("no_campaigns_found")
                                                  : t("no_campaigns_yet", "No campaigns yet")}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            campaigns.map((campaign) => {
                                const campaignColor = getRandomColor(campaign.name ?? campaign.id)
                                const ChannelIcon = channelIcons[campaign.channel] ?? Mail
                                return (
                                    <TableRow
                                        key={campaign.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(campaign)}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                    style={{ backgroundColor: campaignColor }}
                                                >
                                                    <ChannelIcon className="h-4 w-4 text-white" />
                                                </div>
                                                <div>
                                                    <div className="font-medium">
                                                        {campaign.name}
                                                    </div>
                                                    <div className="text-sm text-muted-foreground">
                                                        {snakeToTitle(campaign.channel)}
                                                    </div>
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {campaign.delivery?.sent > 0
                                                ? formatDelivery(campaign.delivery)
                                                : "—"}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, campaign.updated_at, "PP")}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="h-8 w-8 p-0"
                                                        onClick={(e) => e.stopPropagation()}
                                                    >
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem
                                                        onClick={(e) => {
                                                            e.stopPropagation()
                                                            handleRowClick(campaign)
                                                        }}
                                                    >
                                                        <Mail className="mr-2 h-4 w-4" />
                                                        {t("edit")}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={(e) =>
                                                            handleDuplicateCampaign(e, campaign.id)
                                                        }
                                                    >
                                                        <Copy className="mr-2 h-4 w-4" />
                                                        {t("duplicate")}
                                                    </DropdownMenuItem>
                                                    {isArchivedView ? (
                                                        <DropdownMenuItem
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                handleUnarchiveCampaign(
                                                                    campaign.id.toString(),
                                                                )
                                                            }}
                                                        >
                                                            <ArchiveRestore className="mr-2 h-4 w-4" />
                                                            {t("unarchive", "Unarchive")}
                                                        </DropdownMenuItem>
                                                    ) : (
                                                        <DropdownMenuItem
                                                            onClick={(e) =>
                                                                handleArchiveCampaign(
                                                                    e,
                                                                    campaign.id,
                                                                )
                                                            }
                                                            className="text-destructive"
                                                        >
                                                            <Archive className="mr-2 h-4 w-4" />
                                                            {t("archive")}
                                                        </DropdownMenuItem>
                                                    )}
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {campaigns.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {t("campaign.plural").toLowerCase()}
                        </p>
                        {(hasPrevPage || hasNextPage) && (
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handlePrevPage}
                                    disabled={!hasPrevPage}
                                    aria-label={t("previous")}
                                >
                                    <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                    <span className="hidden sm:inline">{t("previous")}</span>
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handleNextPage}
                                    disabled={!hasNextPage}
                                    aria-label={t("next")}
                                >
                                    <span className="hidden sm:inline">{t("next")}</span>
                                    <ChevronRight className="h-4 w-4 sm:ml-1" />
                                </Button>
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Tip Card */}
            <div className="group relative overflow-hidden rounded-lg bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border p-4 sm:p-6">
                <div className="relative z-10 max-w-md">
                    <h3 className="font-semibold text-foreground">
                        {t("campaign_tip_title", "Automate campaigns via API")}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t(
                            "campaign_tip_description",
                            "Trigger campaigns programmatically using the API for automated messaging workflows.",
                        )}
                    </p>
                    <Button
                        variant="link"
                        className="mt-2 h-auto p-0 text-primary"
                        onClick={() => window.open("/api/", "_blank")}
                    >
                        {t("view_api_docs", "View API documentation")}
                        <ArrowRight className="ml-1 h-3 w-3 transition-transform duration-300 group-hover:translate-x-1" />
                    </Button>
                </div>

                {/* Decorative elements */}
                <div className="absolute -right-6 -bottom-6 flex gap-4">
                    <div className="hidden sm:flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 transition-all duration-500 ease-out group-hover:rotate-6 group-hover:-translate-y-2 group-hover:bg-primary/15">
                        <Megaphone
                            className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Mail
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Smartphone
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                </div>
            </div>
        </div>
    )
}
