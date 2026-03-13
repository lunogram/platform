import { useState, useEffect, useMemo, useCallback } from "react"
import { Search, Check, Upload, ImageIcon } from "lucide-react"
import Modal from "@/components/modal"
import { Button } from "@/components/ui/button"
import { cn } from "@/utils"
import { fetchMockImages } from "./mockImages"
import type { Image } from "@/types"

interface ImageWithDescription extends Image {
    description?: string
}

interface ImageLibraryModalProps {
    open: boolean
    onClose: (open: boolean) => void
    /** Callback with the confirmed selection */
    onSelect: (images: ImageWithDescription[]) => void
    /** Pre-selected image IDs when opening */
    selectedIds?: string[]
    /** Single or multi-select mode */
    mode?: "single" | "multi"
}

export function ImageLibraryModal({
    open,
    onClose,
    onSelect,
    selectedIds = [],
    mode = "multi",
}: ImageLibraryModalProps) {
    const [images, setImages] = useState<ImageWithDescription[]>([])
    const [loading, setLoading] = useState(true)
    const [searchQuery, setSearchQuery] = useState("")
    const [localSelection, setLocalSelection] = useState<Set<string>>(new Set(selectedIds))

    // Reset local selection when modal opens
    useEffect(() => {
        if (open) {
            setLocalSelection(new Set(selectedIds))
            setSearchQuery("")
        }
    }, [open, selectedIds])

    // Load images
    useEffect(() => {
        if (!open) return
        setLoading(true)
        fetchMockImages().then((result) => {
            setImages(result)
            setLoading(false)
        })
    }, [open])

    // Filter images by search query (client-side)
    const filteredImages = useMemo(() => {
        if (!searchQuery.trim()) return images
        const q = searchQuery.toLowerCase()
        return images.filter(
            (img) =>
                img.name.toLowerCase().includes(q) ||
                img.filename.toLowerCase().includes(q) ||
                (img.description && img.description.toLowerCase().includes(q)),
        )
    }, [images, searchQuery])

    const toggleImage = useCallback(
        (image: ImageWithDescription) => {
            setLocalSelection((prev) => {
                const next = new Set(prev)
                if (mode === "single") {
                    // In single mode, replace selection
                    if (next.has(image.id)) {
                        next.delete(image.id)
                    } else {
                        next.clear()
                        next.add(image.id)
                    }
                } else {
                    // In multi mode, toggle
                    if (next.has(image.id)) {
                        next.delete(image.id)
                    } else {
                        next.add(image.id)
                    }
                }
                return next
            })
        },
        [mode],
    )

    const handleConfirm = useCallback(() => {
        const selected = images.filter((img) => localSelection.has(img.id))
        onSelect(selected)
        onClose(false)
    }, [images, localSelection, onSelect, onClose])

    const selectionCount = localSelection.size

    return (
        <Modal
            open={open}
            onClose={onClose}
            title="Image Library"
            size="large"
            actions={
                <div className="flex items-center justify-between w-full">
                    <span className="text-sm text-muted-foreground">
                        {selectionCount === 0
                            ? "No images selected"
                            : `${selectionCount} image${selectionCount !== 1 ? "s" : ""} selected`}
                    </span>
                    <div className="flex gap-2">
                        <Button variant="outline" onClick={() => onClose(false)}>
                            Cancel
                        </Button>
                        <Button onClick={handleConfirm} disabled={selectionCount === 0}>
                            {mode === "single" ? "Use image" : "Add to context"}
                        </Button>
                    </div>
                </div>
            }
        >
            <div className="space-y-4">
                {/* Search + Upload bar */}
                <div className="flex items-center gap-2">
                    <div className="relative flex-1">
                        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Search by name or description..."
                            className="w-full rounded-lg border border-input bg-background pl-9 pr-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                            autoFocus
                        />
                    </div>
                    <Button variant="outline" size="sm" className="gap-1.5">
                        <Upload className="w-4 h-4" />
                        Upload
                    </Button>
                </div>

                {/* Image grid */}
                {loading ? (
                    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                        {Array.from({ length: 8 }).map((_, i) => (
                            <div
                                key={i}
                                className="aspect-[4/3] rounded-lg bg-muted animate-pulse"
                            />
                        ))}
                    </div>
                ) : filteredImages.length === 0 ? (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <ImageIcon className="w-10 h-10 text-muted-foreground/50 mb-3" />
                        <p className="text-sm font-medium text-foreground mb-1">No images found</p>
                        <p className="text-xs text-muted-foreground">
                            {searchQuery
                                ? "Try a different search term"
                                : "Upload images to get started"}
                        </p>
                    </div>
                ) : (
                    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3 max-h-[50vh] overflow-y-auto pr-1">
                        {filteredImages.map((image) => {
                            const isSelected = localSelection.has(image.id)
                            return (
                                <button
                                    key={image.id}
                                    type="button"
                                    onClick={() => toggleImage(image)}
                                    className={cn(
                                        "group relative rounded-lg overflow-hidden border-2 transition-all text-left",
                                        isSelected
                                            ? "border-primary ring-1 ring-primary/30"
                                            : "border-transparent hover:border-border",
                                    )}
                                >
                                    {/* Image */}
                                    <div className="aspect-[4/3] bg-muted">
                                        <img
                                            src={image.url}
                                            alt={image.name}
                                            className="w-full h-full object-cover"
                                            loading="lazy"
                                        />
                                    </div>

                                    {/* Selection check */}
                                    <div
                                        className={cn(
                                            "absolute top-2 right-2 w-5 h-5 rounded-full flex items-center justify-center transition-all",
                                            isSelected
                                                ? "bg-primary text-primary-foreground"
                                                : "bg-black/40 text-white opacity-0 group-hover:opacity-100",
                                        )}
                                    >
                                        <Check className="w-3 h-3" />
                                    </div>

                                    {/* Caption */}
                                    <div className="p-2">
                                        <p className="text-xs font-medium text-foreground truncate">
                                            {image.name}
                                        </p>
                                        {image.description && (
                                            <p className="text-xs text-muted-foreground truncate mt-0.5">
                                                {image.description}
                                            </p>
                                        )}
                                    </div>
                                </button>
                            )
                        })}
                    </div>
                )}
            </div>
        </Modal>
    )
}
