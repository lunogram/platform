import { useTranslation } from "react-i18next"
import { Upload } from "lucide-react"
import { useUploadListUsers } from "./listQueries"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

interface ListUploadDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

/** Uploads a file of users into the static list in context. */
export function ListUploadDialog({ open, onOpenChange }: ListUploadDialogProps) {
    const { t } = useTranslation()
    const uploadUsers = useUploadListUsers()

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("import_users")}</DialogTitle>
                    <DialogDescription>{t("upload_instructions")}</DialogDescription>
                </DialogHeader>
                <form
                    onSubmit={(e) => {
                        e.preventDefault()
                        const formData = new FormData(e.currentTarget)
                        const file = formData.get("file") as File
                        // The dialog stays open on failure; the mutation cache reports the error.
                        if (file) uploadUsers.mutate(file, { onSuccess: () => onOpenChange(false) })
                    }}
                >
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label htmlFor="upload-file">{t("file")}</Label>
                            <Input
                                id="upload-file"
                                name="file"
                                type="file"
                                accept=".csv,.txt"
                                required
                                className="cursor-pointer"
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                            {t("cancel")}
                        </Button>
                        <Button type="submit" disabled={uploadUsers.isPending}>
                            <Upload className="mr-2 h-3.5 w-3.5" />
                            {t("upload")}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
