import { useContext } from "react"
import { useTranslation } from "react-i18next"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate } from "../../utils"
import type { components } from "../../oapi/management.generated"

import { JsonView } from "@/components/ui/json-view"
import { TableCell, TableRow } from "@/components/ui/table"

export type OrganizationEvent = components["schemas"]["OrganizationEvent"]

interface EventExpandedRowProps {
    event: OrganizationEvent
}

export function EventExpandedRow({ event }: EventExpandedRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const hasData = event.data && Object.keys(event.data).length > 0

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={4} className="p-0">
                <div className="px-4 sm:px-6 py-4 space-y-4">
                    {/* Event Info */}
                    <div className="flex flex-col sm:flex-row gap-4 sm:gap-8">
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("event_id")}
                            </p>
                            <code className="text-sm">{event.id}</code>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("timestamp")}
                            </p>
                            <p className="text-sm">
                                {formatDate(preferences, event.created_at, "PPpp")}
                            </p>
                        </div>
                    </div>

                    {/* Event Data */}
                    {hasData && (
                        <JsonView
                            data={event.data as Record<string, unknown>}
                            title={t("event_data")}
                            defaultExpanded
                        />
                    )}
                </div>
            </TableCell>
        </TableRow>
    )
}
