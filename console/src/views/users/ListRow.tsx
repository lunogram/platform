import { useContext } from "react"
import { useTranslation } from "react-i18next"
import { Archive, ArchiveRestore, Copy, ListFilter, MoreHorizontal } from "lucide-react"
import type { components } from "@/oapi/management.generated"
import type { ListState } from "@/types"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { formatDate, snakeToTitle } from "@/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { TableCell, TableRow } from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

type ListItem = components["schemas"]["List"]

function getStateBadge(state: ListState, t: (key: string) => string) {
    const config: Record<ListState, { label: string; className: string }> = {
        draft: { label: t("draft"), className: "bg-secondary text-secondary-foreground" },
        loading: { label: t("loading"), className: "bg-blue-100 text-blue-700" },
        ready: { label: t("ready"), className: "bg-green-100 text-green-700" },
    }
    const { label, className } = config[state] ?? config.draft
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {label}
        </Badge>
    )
}

interface ListRowProps {
    list: ListItem
    /** The archived view offers unarchive instead of archive. */
    archived: boolean
    onOpen: () => void
    onDuplicate: (e: React.MouseEvent) => void
    onArchive: (e: React.MouseEvent) => void
    onUnarchive: () => void
}

export function ListRow({
    list,
    archived,
    onOpen,
    onDuplicate,
    onArchive,
    onUnarchive,
}: ListRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const listColor = getRandomColor(list.name ?? list.id)
    return (
        <TableRow className="cursor-pointer" onClick={onOpen}>
            <TableCell>
                <div className="flex items-center gap-3">
                    <div
                        className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                        style={{ backgroundColor: listColor }}
                    >
                        <ListFilter className="h-4 w-4 text-white" />
                    </div>
                    <div className="font-medium">{list.name}</div>
                </div>
            </TableCell>
            <TableCell className="hidden sm:table-cell text-muted-foreground">
                {snakeToTitle(list.type)}
            </TableCell>
            <TableCell className="hidden sm:table-cell text-muted-foreground">
                {list.users_count?.toLocaleString() ?? "—"}
            </TableCell>
            <TableCell>{getStateBadge(list.state, t)}</TableCell>
            <TableCell className="hidden md:table-cell text-muted-foreground">
                {formatDate(preferences, list.created_at, "PP")}
            </TableCell>
            <TableCell className="hidden md:table-cell text-muted-foreground">
                {formatDate(preferences, list.updated_at, "PP")}
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
                            <ListFilter className="mr-2 h-4 w-4" />
                            {t("edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={onDuplicate}>
                            <Copy className="mr-2 h-4 w-4" />
                            {t("duplicate")}
                        </DropdownMenuItem>
                        {archived ? (
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
