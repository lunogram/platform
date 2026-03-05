import { useState, useRef } from "react"
import { useTranslation, Trans } from "react-i18next"
import { Upload, FileText, X, Download } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

// --- Shared upload form (used in both dialog and onboarding) ---

interface UserImportFormProps {
    file: File | null
    onFileChange: (file: File | null) => void
}

export function UserImportForm({ file, onFileChange }: UserImportFormProps) {
    const { t } = useTranslation()
    const [isDragging, setIsDragging] = useState(false)
    const fileInputRef = useRef<HTMLInputElement>(null)

    const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
        const selected = e.target.files?.[0]
        if (selected) onFileChange(selected)
    }

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault()
        e.stopPropagation()
        setIsDragging(false)
        const dropped = e.dataTransfer.files[0]
        if (dropped) onFileChange(dropped)
    }

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault()
        e.stopPropagation()
        setIsDragging(true)
    }

    const handleDragLeave = (e: React.DragEvent) => {
        e.preventDefault()
        e.stopPropagation()
        setIsDragging(false)
    }

    const clearFile = (e: React.MouseEvent) => {
        e.stopPropagation()
        onFileChange(null)
        if (fileInputRef.current) fileInputRef.current.value = ""
    }

    return (
        <div className="grid gap-4">
            {/* Template download */}
            <div className="flex items-center gap-2 rounded-md border border-dashed p-3 text-sm text-muted-foreground">
                <Download className="h-4 w-4 shrink-0" />
                <Trans
                    i18nKey="onboarding_project_users_template"
                    components={{
                        download: (
                            <a
                                href="/templates/users.csv"
                                download="users.csv"
                                className="font-medium text-primary underline underline-offset-4 hover:text-primary/80"
                            />
                        ),
                    }}
                />
            </div>

            {/* Drop zone */}
            <div
                className={`relative flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-8 transition-colors cursor-pointer ${
                    isDragging
                        ? "border-primary bg-primary/5"
                        : file
                          ? "border-primary/50 bg-primary/5"
                          : "border-muted-foreground/25 hover:border-muted-foreground/50"
                }`}
                onClick={() => fileInputRef.current?.click()}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
            >
                <input
                    ref={fileInputRef}
                    type="file"
                    accept=".csv,text/csv"
                    onChange={handleFileInput}
                    className="hidden"
                />
                {file ? (
                    <>
                        <FileText className="h-8 w-8 text-primary/60" />
                        <div className="text-center">
                            <p className="text-sm font-medium">{file.name}</p>
                            <p className="text-xs text-muted-foreground">
                                {(file.size / 1024).toFixed(1)} KB
                            </p>
                        </div>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="absolute right-2 top-2 h-6 w-6 p-0"
                            onClick={clearFile}
                        >
                            <X className="h-3 w-3" />
                        </Button>
                    </>
                ) : (
                    <>
                        <Upload className="h-8 w-8 text-muted-foreground/50" />
                        <div className="text-center">
                            <p className="text-sm text-muted-foreground">
                                {t("drop_csv_here", "Drop a CSV file here, or click to browse")}
                            </p>
                        </div>
                    </>
                )}
            </div>
        </div>
    )
}

// --- Dialog wrapper (used on the Users page) ---

interface UserImportDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    onImport: (file: File) => Promise<void>
}

export function UserImportDialog({ open, onOpenChange, onImport }: UserImportDialogProps) {
    const { t } = useTranslation()
    const [file, setFile] = useState<File | null>(null)
    const [isImporting, setIsImporting] = useState(false)

    const handleOpenChange = (open: boolean) => {
        if (!open) {
            setFile(null)
            setIsImporting(false)
        }
        onOpenChange(open)
    }

    const handleImport = async () => {
        if (!file) return
        setIsImporting(true)
        try {
            await onImport(file)
            handleOpenChange(false)
        } finally {
            setIsImporting(false)
        }
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("import_users", "Import Users")}</DialogTitle>
                    <DialogDescription>{t("upload_instructions")}</DialogDescription>
                </DialogHeader>
                <div className="py-4">
                    <UserImportForm file={file} onFileChange={setFile} />
                </div>
                <DialogFooter>
                    <Button
                        variant="outline"
                        onClick={() => handleOpenChange(false)}
                        disabled={isImporting}
                    >
                        {t("cancel")}
                    </Button>
                    <Button onClick={handleImport} disabled={!file || isImporting}>
                        {isImporting ? (
                            t("importing", "Importing...")
                        ) : (
                            <>
                                <Upload className="mr-2 h-4 w-4" />
                                {t("import", "Import")}
                            </>
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
