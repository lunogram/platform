import { useCallback, useContext, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import {
    ImageIcon,
    Search,
    Upload,
    Loader2,
    Trash2,
    FileImage,
    HardDriveDownload,
} from "lucide-react"

import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"

import { ProjectContext } from "@/contexts"
import api from "@/api"
import { oapiClient } from "@/oapi/client"
import type { Image } from "@/types"
import { cn } from "@/utils"

/* ------------------------------------------------------------------ */
/*  Public interface                                                   */
/* ------------------------------------------------------------------ */

export interface MediaManagerProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    /** Called when the user picks an image. Closes the dialog automatically. */
    onSelect: (image: Image) => void
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function MediaManager({ open, onOpenChange, onSelect }: MediaManagerProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [images, setImages] = useState<Image[]>([])
    const [loading, setLoading] = useState(false)
    const [uploading, setUploading] = useState(false)
    const [searchQuery, setSearchQuery] = useState("")
    const [hasLoaded, setHasLoaded] = useState(false)
    const [dragOver, setDragOver] = useState(false)
    const [deletingId, setDeletingId] = useState<string | null>(null)

    const fileInputRef = useRef<HTMLInputElement>(null)

    /* ---- data loading ---- */

    const loadImages = useCallback(
        async (query?: string) => {
            setLoading(true)
            try {
                const { data, error } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/documents",
                    {
                        params: {
                            path: { projectID: project.id },
                            query: {
                                search: query || undefined,
                                limit: 50,
                            } as { limit: number; search?: string },
                        },
                    },
                )
                if (error) throw error
                setImages((data?.results ?? []) as Image[])
                setHasLoaded(true)
            } catch {
                setImages([])
            } finally {
                setLoading(false)
            }
        },
        [project.id],
    )

    // Load on first open
    useEffect(() => {
        if (open && !hasLoaded) {
            loadImages()
        }
    }, [open, hasLoaded, loadImages])

    // Reset state when the dialog closes
    useEffect(() => {
        if (!open) {
            setSearchQuery("")
            setHasLoaded(false)
        }
    }, [open])

    /* ---- search ---- */

    const handleSearch = useCallback(() => {
        loadImages(searchQuery)
    }, [loadImages, searchQuery])

    const handleSearchKeyDown = useCallback(
        (e: React.KeyboardEvent) => {
            if (e.key === "Enter") handleSearch()
        },
        [handleSearch],
    )

    /* ---- upload ---- */

    const uploadFile = useCallback(
        async (file: File) => {
            if (!file.type.startsWith("image/")) return

            setUploading(true)
            try {
                await api.images.create(project.id, file)
                await loadImages(searchQuery)
            } catch {
                // Upload failed – swallow for now
            } finally {
                setUploading(false)
            }
        },
        [project.id, loadImages, searchQuery],
    )

    const handleFileSelect = useCallback(
        (e: React.ChangeEvent<HTMLInputElement>) => {
            const file = e.target.files?.[0]
            if (file) uploadFile(file)
            e.target.value = ""
        },
        [uploadFile],
    )

    /* ---- drag & drop ---- */

    const handleDragOver = useCallback((e: React.DragEvent) => {
        e.preventDefault()
        setDragOver(true)
    }, [])

    const handleDragLeave = useCallback((e: React.DragEvent) => {
        e.preventDefault()
        setDragOver(false)
    }, [])

    const handleDrop = useCallback(
        (e: React.DragEvent) => {
            e.preventDefault()
            setDragOver(false)
            const file = e.dataTransfer.files?.[0]
            if (file) uploadFile(file)
        },
        [uploadFile],
    )

    /* ---- delete ---- */

    const handleDelete = useCallback(
        async (e: React.MouseEvent, imageId: string) => {
            e.stopPropagation()
            setDeletingId(imageId)
            try {
                const { error } = await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/documents/{documentID}",
                    { params: { path: { projectID: project.id, documentID: imageId } } },
                )
                if (error) throw error
                setImages((prev) => prev.filter((img) => img.id !== imageId))
            } catch {
                // Delete failed – swallow for now
            } finally {
                setDeletingId(null)
            }
        },
        [project.id],
    )

    /* ---- selection ---- */

    const handleSelect = useCallback(
        (image: Image) => {
            onSelect(image)
            onOpenChange(false)
        },
        [onSelect, onOpenChange],
    )

    /* ---- helpers ---- */

    const formatBytes = (bytes: number): string => {
        if (bytes === 0) return "0 B"
        const k = 1024
        const sizes = ["B", "KB", "MB"]
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
    }

    /* ---- render ---- */

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-3xl max-h-[80vh] flex flex-col">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <ImageIcon className="h-4 w-4" />
                        {t("mediaManager.title", "Media Manager")}
                    </DialogTitle>
                    <DialogDescription>
                        {t(
                            "mediaManager.description",
                            "Upload, manage, and select images for your email templates.",
                        )}
                    </DialogDescription>
                </DialogHeader>

                {/* Search + Upload bar */}
                <div className="flex items-center gap-2">
                    <div className="relative flex-1">
                        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                        <Input
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            onKeyDown={handleSearchKeyDown}
                            placeholder={t("mediaManager.search", "Search images...")}
                            className="pl-8 h-8 text-sm"
                        />
                    </div>
                    <Button
                        variant="outline"
                        size="sm"
                        className="h-8 gap-1.5"
                        onClick={() => fileInputRef.current?.click()}
                        disabled={uploading}
                    >
                        {uploading ? (
                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                            <Upload className="h-3.5 w-3.5" />
                        )}
                        {t("mediaManager.upload", "Upload")}
                    </Button>
                    <input
                        ref={fileInputRef}
                        type="file"
                        accept="image/*"
                        className="hidden"
                        onChange={handleFileSelect}
                    />
                </div>

                {/* Image grid */}
                <div
                    className={cn(
                        "flex-1 min-h-0 rounded-md border transition-colors",
                        dragOver ? "border-primary bg-primary/5" : "border-border",
                    )}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                >
                    {loading ? (
                        <div className="flex items-center justify-center h-80">
                            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                        </div>
                    ) : images.length === 0 ? (
                        <div className="flex flex-col items-center justify-center h-80 gap-3 text-muted-foreground">
                            <HardDriveDownload className="h-10 w-10" />
                            <div className="text-center">
                                <p className="text-sm font-medium">
                                    {t("mediaManager.emptyTitle", "No images yet")}
                                </p>
                                <p className="text-xs mt-1">
                                    {t(
                                        "mediaManager.emptyDescription",
                                        "Drag and drop images here, or click Upload to get started.",
                                    )}
                                </p>
                            </div>
                        </div>
                    ) : (
                        <ScrollArea className="h-80">
                            <div className="grid grid-cols-4 gap-2 p-2">
                                {images.map((image) => (
                                    <button
                                        key={image.id}
                                        type="button"
                                        className="group relative aspect-square rounded-md overflow-hidden border bg-muted/30 hover:ring-2 hover:ring-primary transition-all cursor-pointer"
                                        onClick={() => handleSelect(image)}
                                    >
                                        <img
                                            src={image.url}
                                            alt={image.name || image.filename}
                                            className="w-full h-full object-cover"
                                            loading="lazy"
                                        />

                                        {/* Hover overlay with info */}
                                        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition-colors" />

                                        {/* Delete button */}
                                        <button
                                            type="button"
                                            className="absolute top-1.5 right-1.5 h-6 w-6 rounded-md bg-black/60 text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive"
                                            onClick={(e) => handleDelete(e, image.id)}
                                            disabled={deletingId === image.id}
                                        >
                                            {deletingId === image.id ? (
                                                <Loader2 className="h-3 w-3 animate-spin" />
                                            ) : (
                                                <Trash2 className="h-3 w-3" />
                                            )}
                                        </button>

                                        {/* File info footer */}
                                        <div className="absolute inset-x-0 bottom-0 bg-black/60 px-2 py-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                            <p className="text-[10px] text-white truncate font-medium">
                                                {image.filename || image.name}
                                            </p>
                                            <div className="flex items-center gap-1.5 mt-0.5">
                                                <FileImage className="h-2.5 w-2.5 text-white/70" />
                                                <span className="text-[9px] text-white/70">
                                                    {image.content_type
                                                        ?.split("/")[1]
                                                        ?.toUpperCase() ?? "IMG"}
                                                </span>
                                                {image.size_bytes > 0 && (
                                                    <span className="text-[9px] text-white/70">
                                                        {formatBytes(image.size_bytes)}
                                                    </span>
                                                )}
                                            </div>
                                        </div>
                                    </button>
                                ))}
                            </div>
                        </ScrollArea>
                    )}
                </div>

                {/* Footer with count */}
                {images.length > 0 && (
                    <div className="text-xs text-muted-foreground">
                        {t("mediaManager.imageCount", "{{count}} image(s)", {
                            count: images.length,
                        })}
                    </div>
                )}
            </DialogContent>
        </Dialog>
    )
}
