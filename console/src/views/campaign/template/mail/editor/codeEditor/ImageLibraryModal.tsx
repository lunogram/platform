import { useCallback, useContext, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ImageIcon, Search, Upload, Loader2 } from "lucide-react"

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
import type { Image } from "@/types"

interface ImageLibraryModalProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    onInsert: (url: string, alt: string) => void
}

export function ImageLibraryModal({ open, onOpenChange, onInsert }: ImageLibraryModalProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [images, setImages] = useState<Image[]>([])
    const [loading, setLoading] = useState(false)
    const [uploading, setUploading] = useState(false)
    const [searchQuery, setSearchQuery] = useState("")
    const [hasSearched, setHasSearched] = useState(false)
    const [dragOver, setDragOver] = useState(false)

    const fileInputRef = useRef<HTMLInputElement>(null)

    // Load images
    const loadImages = useCallback(
        async (query?: string) => {
            setLoading(true)
            try {
                const result = await api.images.search(project.id, {
                    search: query || undefined,
                    limit: 50,
                })
                setImages(result.results ?? result.data ?? [])
                setHasSearched(true)
            } catch {
                setImages([])
            } finally {
                setLoading(false)
            }
        },
        [project.id],
    )

    // Load images when modal opens
    useEffect(() => {
        if (open && !hasSearched) {
            loadImages()
        }
    }, [open, hasSearched, loadImages])

    // Search handler
    const handleSearch = useCallback(() => {
        loadImages(searchQuery)
    }, [loadImages, searchQuery])

    const handleSearchKeyDown = useCallback(
        (e: React.KeyboardEvent) => {
            if (e.key === "Enter") {
                handleSearch()
            }
        },
        [handleSearch],
    )

    // Upload handler
    const uploadFile = useCallback(
        async (file: File) => {
            if (!file.type.startsWith("image/")) return

            setUploading(true)
            try {
                await api.images.create(project.id, file)
                // Reload the list to show the new image
                await loadImages(searchQuery)
            } catch {
                // Upload failed silently
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
            // Reset input so the same file can be re-selected
            e.target.value = ""
        },
        [uploadFile],
    )

    // Drag and drop handlers
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

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <ImageIcon className="h-4 w-4" />
                        {t("campaign.template.email.editor.imageLibrary", "Image Library")}
                    </DialogTitle>
                    <DialogDescription>
                        {t(
                            "campaign.template.email.editor.imageLibraryDescription",
                            "Upload or select an image to insert into your template.",
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
                            placeholder={t(
                                "campaign.template.email.editor.searchImages",
                                "Search images...",
                            )}
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
                        {t("campaign.template.email.editor.upload", "Upload")}
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
                    className={`flex-1 min-h-0 rounded-md border transition-colors ${
                        dragOver ? "border-primary bg-primary/5" : "border-border"
                    }`}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                >
                    {loading ? (
                        <div className="flex items-center justify-center h-64">
                            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                        </div>
                    ) : images.length === 0 ? (
                        <div className="flex flex-col items-center justify-center h-64 gap-2 text-muted-foreground">
                            <Upload className="h-8 w-8" />
                            <p className="text-sm">
                                {t(
                                    "campaign.template.email.editor.dropImages",
                                    "Drag and drop images here, or click Upload",
                                )}
                            </p>
                        </div>
                    ) : (
                        <ScrollArea className="h-64">
                            <div className="grid grid-cols-4 gap-2 p-2">
                                {images.map((image) => (
                                    <button
                                        key={image.id}
                                        type="button"
                                        className="group relative aspect-square rounded-md overflow-hidden border bg-muted/30 hover:ring-2 hover:ring-primary transition-all cursor-pointer"
                                        onClick={() =>
                                            onInsert(image.url, image.name || image.filename)
                                        }
                                    >
                                        <img
                                            src={image.url}
                                            alt={image.name || image.filename}
                                            className="w-full h-full object-cover"
                                            loading="lazy"
                                        />
                                        <div className="absolute inset-x-0 bottom-0 bg-black/60 px-1.5 py-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                            <p className="text-[10px] text-white truncate">
                                                {image.filename || image.name}
                                            </p>
                                        </div>
                                    </button>
                                ))}
                            </div>
                        </ScrollArea>
                    )}
                </div>
            </DialogContent>
        </Dialog>
    )
}
