import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { TableCell, TableRow } from "@/components/ui/table"

interface LoadErrorRowProps {
    colSpan: number
    message: string
    onRetry: () => void
}

/** A table row saying the rows failed to load, so a failure never reads as "nothing here". */
export function LoadErrorRow({ colSpan, message, onRetry }: LoadErrorRowProps) {
    const { t } = useTranslation()
    return (
        <TableRow>
            <TableCell colSpan={colSpan} className="h-32 text-center">
                <div className="flex flex-col items-center gap-3 text-muted-foreground">
                    <p>{message}</p>
                    <Button variant="outline" size="sm" onClick={onRetry}>
                        {t("retry", "Retry")}
                    </Button>
                </div>
            </TableCell>
        </TableRow>
    )
}
