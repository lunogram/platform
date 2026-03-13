import { useState, useCallback, useRef, useEffect, type KeyboardEvent, type ReactNode } from "react"
import { Send, X, Crosshair, ImageIcon, ImagePlus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useBuilderActions, useBuilderThread } from "./useBuilder"

interface PromptInputProps {
    /** Autofocus the textarea on mount */
    autoFocus?: boolean
    /** Callback to open the image library modal */
    onAddImages?: () => void
    /** Placeholder text for the textarea */
    placeholder?: string
    /** Variant for the image button */
    imageButtonVariant?: "outline" | "ghost"
    /** Additional class for the context pills wrapper */
    pillsClassName?: string
    /** Additional class for the textarea element */
    textareaClassName?: string
    /** Wrap the input row (textarea + buttons) in a custom container */
    inputRowClassName?: string
}

export function PromptInput({
    autoFocus = true,
    onAddImages,
    placeholder = "Describe the email you want to create...",
    imageButtonVariant = "outline",
    pillsClassName = "",
    textareaClassName = "flex-1 resize-none rounded-lg border border-input bg-background px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50",
    inputRowClassName = "flex items-end gap-2",
}: PromptInputProps) {
    const { sendMessage, deselectSection, deselectImage } = useBuilderActions()
    const { selectedSections, selectedImages, isAgentTyping } = useBuilderThread()
    const [text, setText] = useState("")
    const textareaRef = useRef<HTMLTextAreaElement>(null)

    const canSend = text.trim().length > 0 && !isAgentTyping

    // Auto-resize textarea
    useEffect(() => {
        const el = textareaRef.current
        if (el) {
            el.style.height = "auto"
            el.style.height = `${Math.min(el.scrollHeight, 160)}px`
        }
    }, [text])

    const handleSend = useCallback(async () => {
        if (!canSend) return
        const message = text.trim()
        setText("")
        await sendMessage(message)
    }, [canSend, text, sendMessage])

    const handleKeyDown = useCallback(
        (e: KeyboardEvent<HTMLTextAreaElement>) => {
            if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault()
                handleSend()
            }
        },
        [handleSend],
    )

    const hasPills = selectedSections.length > 0 || selectedImages.length > 0

    return (
        <>
            {/* Context pills */}
            {hasPills && (
                <div className={`flex flex-wrap gap-1.5 ${pillsClassName}`}>
                    {/* Section pills */}
                    {selectedSections.map((section) => (
                        <Badge
                            key={section.label}
                            variant="secondary"
                            className="gap-1 font-normal max-w-full bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20"
                        >
                            <Crosshair className="h-3 w-3 shrink-0" />
                            <span className="truncate">
                                {section.label.length > 15
                                    ? section.label.slice(0, 15) + "…"
                                    : section.label}
                            </span>
                            <button
                                type="button"
                                onClick={() => deselectSection(section.label)}
                                className="ml-0.5 shrink-0 cursor-pointer hover:text-blue-800 dark:hover:text-blue-200"
                            >
                                <X className="h-3 w-3" />
                            </button>
                        </Badge>
                    ))}

                    {/* Image pills */}
                    {selectedImages.map((image) => (
                        <Tooltip key={image.id}>
                            <TooltipTrigger asChild>
                                <Badge
                                    variant="secondary"
                                    className="gap-1 font-normal max-w-full bg-primary/10 text-primary border-primary/20"
                                >
                                    <ImageIcon className="h-3 w-3 shrink-0" />
                                    <span className="truncate">{image.name}</span>
                                    <button
                                        type="button"
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            deselectImage(image.id)
                                        }}
                                        className="ml-0.5 shrink-0 cursor-pointer hover:text-foreground"
                                    >
                                        <X className="h-3 w-3" />
                                    </button>
                                </Badge>
                            </TooltipTrigger>
                            <TooltipContent side="top" className="p-1">
                                <img
                                    src={image.url}
                                    alt={image.name}
                                    className="rounded max-w-48 max-h-32 object-cover"
                                />
                            </TooltipContent>
                        </Tooltip>
                    ))}
                </div>
            )}

            {/* Input row */}
            <div className={inputRowClassName}>
                {/* Add images button */}
                {onAddImages && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                type="button"
                                variant={imageButtonVariant}
                                size="sm"
                                className="h-9 w-9 p-0 flex-shrink-0"
                                onClick={onAddImages}
                            >
                                <ImagePlus className="h-4 w-4" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>Add images</TooltipContent>
                    </Tooltip>
                )}

                <textarea
                    ref={textareaRef}
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={placeholder}
                    rows={1}
                    autoFocus={autoFocus}
                    className={textareaClassName}
                />

                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button
                            type="button"
                            size="sm"
                            className="h-9 w-9 p-0 flex-shrink-0"
                            onClick={handleSend}
                            disabled={!canSend}
                        >
                            <Send className="h-4 w-4" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>Send message</TooltipContent>
                </Tooltip>
            </div>
        </>
    )
}
