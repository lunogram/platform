import { useContext } from "react"
import { useTranslation } from "react-i18next"
import { Archive, ArchiveRestore, Copy, GitBranch, MoreHorizontal } from "lucide-react"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { formatDate } from "@/utils"
import type { Journey } from "@/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { TableCell, TableRow } from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

type JourneyStatus = "draft" | "published" | "archived"

/** The fields of a journey the list shows; active and archived lists share them. */
export type JourneyListItem = Pick<
    Journey,
    "id" | "name" | "description" | "status" | "created_at" | "updated_at"
>

function getStatusBadge(status: JourneyStatus, t: (key: string) => string) {
    const config: Record<JourneyStatus, { label: string; className: string }> = {
        draft: { label: t("draft"), className: "bg-secondary text-secondary-foreground" },
        published: { label: t("published"), className: "bg-green-100 text-green-700" },
        archived: { label: t("archived"), className: "bg-secondary text-secondary-foreground" },
    }
    const { label, className } = config[status] ?? config.draft
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {label}
        </Badge>
    )
}

interface JourneyRowProps {
    journey: JourneyListItem
    isArchivedView: boolean
    onOpen: () => void
    onDuplicate: (e: React.MouseEvent) => void
    onArchive: (e: React.MouseEvent) => void
    onUnarchive: () => void
}

export function JourneyRow({
    journey,
    isArchivedView,
    onOpen,
    onDuplicate,
    onArchive,
    onUnarchive,
}: JourneyRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const journeyColor = getRandomColor(journey.name ?? journey.id)

    return (
        <TableRow className="cursor-pointer" onClick={onOpen}>
            <TableCell>
                <button
                    type="button"
                    className="flex w-full items-center gap-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    onClick={(e) => {
                        e.stopPropagation()
                        onOpen()
                    }}
                >
                    <div
                        className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                        style={{ backgroundColor: journeyColor }}
                    >
                        <GitBranch className="h-4 w-4 text-white" />
                    </div>
                    <div>
                        <div className="font-medium">{journey.name}</div>
                        {journey.description && (
                            <div className="text-sm text-muted-foreground truncate max-w-[300px]">
                                {journey.description}
                            </div>
                        )}
                    </div>
                </button>
            </TableCell>
            <TableCell>{getStatusBadge(journey.status as JourneyStatus, t)}</TableCell>
            <TableCell className="hidden sm:table-cell text-muted-foreground">
                {formatDate(preferences, journey.created_at, "PP")}
            </TableCell>
            <TableCell className="hidden md:table-cell text-muted-foreground">
                {formatDate(preferences, journey.updated_at, "PP")}
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
                                onOpen()
                            }}
                        >
                            <GitBranch className="mr-2 h-4 w-4" />
                            {t("edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={onDuplicate}>
                            <Copy className="mr-2 h-4 w-4" />
                            {t("duplicate")}
                        </DropdownMenuItem>
                        {isArchivedView ? (
                            <DropdownMenuItem
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onUnarchive()
                                }}
                            >
                                <ArchiveRestore className="mr-2 h-4 w-4" />
                                {t("unarchive", "Unarchive")}
                            </DropdownMenuItem>
                        ) : (
                            <DropdownMenuItem onClick={onArchive} className="text-destructive">
                                <Archive className="mr-2 h-4 w-4" />
                                {t("archive")}
                            </DropdownMenuItem>
                        )}
                    </DropdownMenuContent>
                </DropdownMenu>
            </TableCell>
        </TableRow>
    )
}
