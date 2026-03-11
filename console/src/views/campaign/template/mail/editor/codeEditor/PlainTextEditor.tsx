import { forwardRef, useImperativeHandle, useRef } from "react"
import { useTranslation } from "react-i18next"
import { Sparkles, Undo2 } from "lucide-react"

import { cn } from "@/utils"
import { EditorToolbar } from "./EditorToolbar"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

export interface PlainTextEditorRef {
    insertAtCursor: (text: string) => void
}

interface PlainTextEditorProps {
    autoText: string
    customText: string
    onCustomTextChange: (text: string) => void
    useCustom: boolean
    onToggleCustom: (value: boolean) => void
    onImageClick: () => void
    onInsertVariable: (path: string) => void
    variableGroups: VariableGroup[]
}

export const PlainTextEditor = forwardRef<PlainTextEditorRef, PlainTextEditorProps>(
    function PlainTextEditor(
        {
            autoText,
            customText,
            onCustomTextChange,
            useCustom,
            onToggleCustom,
            onImageClick,
            onInsertVariable,
            variableGroups,
        },
        ref,
    ) {
        const { t } = useTranslation()
        const textareaRef = useRef<HTMLTextAreaElement>(null)

        const displayText = useCustom ? customText : autoText

        // Expose insertAtCursor method via ref
        useImperativeHandle(ref, () => ({
            insertAtCursor: (text: string) => {
                const textarea = textareaRef.current
                if (!textarea) return

                const start = textarea.selectionStart
                const end = textarea.selectionEnd
                const currentText = displayText

                // Insert text at cursor position
                const newText = currentText.slice(0, start) + text + currentText.slice(end)

                // If we're in auto mode, switch to custom mode
                if (!useCustom) {
                    onToggleCustom(true)
                }
                onCustomTextChange(newText)

                // Restore focus and set cursor position after the inserted text
                requestAnimationFrame(() => {
                    textarea.focus()
                    const newCursorPos = start + text.length
                    textarea.setSelectionRange(newCursorPos, newCursorPos)
                })
            },
        }))

        const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
            const newValue = e.target.value
            if (!useCustom) {
                // Auto-switch to custom mode when user starts editing
                onToggleCustom(true)
                onCustomTextChange(newValue)
            } else {
                onCustomTextChange(newValue)
            }
        }

        const handleResetToAuto = () => {
            onToggleCustom(false)
            onCustomTextChange("")
        }

        return (
            <div className="flex flex-col h-full relative">
                {/* Floating toolbar - only show variables button for plain text */}
                <EditorToolbar
                    onImageClick={onImageClick}
                    onInsertVariable={onInsertVariable}
                    variableGroups={variableGroups}
                    showImageButton={false}
                />

                {/* Text area */}
                <textarea
                    ref={textareaRef}
                    className="w-full h-full resize-none p-4 font-mono text-sm leading-relaxed focus:outline-none bg-background text-foreground placeholder:text-muted-foreground"
                    value={displayText}
                    onChange={handleTextChange}
                    placeholder={t(
                        "campaign.template.email.editor.plainTextAutoPlaceholder",
                        "Plain text will be generated from your template...",
                    )}
                    spellCheck={useCustom}
                />

                {/* Bottom-right status indicator */}
                {displayText && (
                    <div className="absolute bottom-3 right-3">
                        {useCustom ? (
                            <button
                                onClick={handleResetToAuto}
                                className={cn(
                                    "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md",
                                    "bg-primary text-primary-foreground shadow-sm",
                                    "text-xs",
                                    "hover:bg-primary/90",
                                    "transition-colors cursor-pointer",
                                )}
                            >
                                <Undo2 className="h-3 w-3" />
                                <span className="font-medium">
                                    {t(
                                        "campaign.template.email.editor.revertToAuto",
                                        "Revert to auto",
                                    )}
                                </span>
                            </button>
                        ) : (
                            <div className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-muted/90 backdrop-blur-sm">
                                <Sparkles className="h-3 w-3 text-muted-foreground" />
                                <span className="text-xs text-muted-foreground font-medium">
                                    {t(
                                        "campaign.template.email.editor.autoGeneratedLabel",
                                        "Auto-generated",
                                    )}
                                </span>
                            </div>
                        )}
                    </div>
                )}
            </div>
        )
    },
)
