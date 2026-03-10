import { useState, useEffect, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Code2, Undo2, Redo2, Braces, ChevronDown } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import type { editor } from "monaco-editor"
import type CodeStore from "../CodeEditorPlugins/CodeStore"
import { useCampaignVariableContext } from "@/views/campaign/CampaignVariableContext"

interface HtmlEditorHeaderProps {
    editorRef: React.MutableRefObject<editor.IStandaloneCodeEditor | null>
    codeStore: typeof CodeStore
}

export function HtmlEditorHeader({ editorRef }: HtmlEditorHeaderProps) {
    const { t } = useTranslation()
    const { variableGroups } = useCampaignVariableContext()
    const [, setCanUndo] = useState(false)
    const [, setCanRedo] = useState(false)
    const [varPickerOpen, setVarPickerOpen] = useState(false)

    const hasVariables = variableGroups.some((g) => g.variables.length > 0)

    // Update undo/redo state when editor content changes
    const updateUndoRedoState = useCallback(() => {
        const editor = editorRef.current
        if (editor) {
            const model = editor.getModel()
            if (model) {
                setCanUndo(true)
                setCanRedo(false)
            }
        }
    }, [editorRef])

    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            const disposable = editor.onDidChangeModelContent(() => {
                updateUndoRedoState()
            })
            return () => disposable.dispose()
        }
    }, [editorRef, updateUndoRedoState])

    const handleUndo = () => {
        const editor = editorRef.current
        if (editor) {
            editor.trigger("keyboard", "undo", null)
            editor.focus()
        }
    }

    const handleRedo = () => {
        const editor = editorRef.current
        if (editor) {
            editor.trigger("keyboard", "redo", null)
            editor.focus()
        }
    }

    const insertVariable = useCallback(
        (path: string) => {
            const editor = editorRef.current
            if (editor) {
                const snippet = `{{ ${path} }}`
                const selection = editor.getSelection()
                if (selection) {
                    editor.executeEdits("insert-variable", [
                        {
                            range: selection,
                            text: snippet,
                            forceMoveMarkers: true,
                        },
                    ])
                    editor.focus()
                }
            }
            setVarPickerOpen(false)
        },
        [editorRef],
    )

    return (
        <div className="flex items-center justify-between px-4 py-3 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
            <div className="flex items-center gap-3">
                {/* Editor Mode Indicator */}
                <div className="flex items-center gap-2">
                    <Code2 className="h-4 w-4 text-primary shrink-0" />
                    <span className="text-sm font-medium whitespace-nowrap">
                        {t("campaign.template.email.editor.developerMode", "Developer Mode")}
                    </span>
                </div>

                {/* Divider */}
                <div className="h-5 w-px bg-border shrink-0" />

                {/* Undo/Redo */}
                <div className="flex items-center">
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button variant="ghost" size="sm" onClick={handleUndo}>
                                <Undo2 className="h-4 w-4" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t("common.undo", "Undo")} (Ctrl+Z)</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button variant="ghost" size="sm" onClick={handleRedo}>
                                <Redo2 className="h-4 w-4" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t("common.redo", "Redo")} (Ctrl+Y)</TooltipContent>
                    </Tooltip>
                </div>

                {/* Template Variables Picker */}
                {hasVariables && (
                    <Popover open={varPickerOpen} onOpenChange={setVarPickerOpen}>
                        <PopoverTrigger asChild>
                            <Button variant="ghost" size="sm" className="gap-1.5">
                                <Braces className="h-4 w-4" />
                                <ChevronDown className="h-3 w-3 opacity-50" />
                            </Button>
                        </PopoverTrigger>
                        <PopoverContent
                            className="w-72 p-0"
                            align="start"
                            side="bottom"
                            onOpenAutoFocus={(e) => e.preventDefault()}
                        >
                            <Command>
                                <CommandInput placeholder="Search variables..." />
                                <CommandList>
                                    <CommandEmpty>No variables found.</CommandEmpty>
                                    {variableGroups.map((group) => (
                                        <CommandGroup key={group.label} heading={group.label}>
                                            {group.variables.map((v) => (
                                                <CommandItem
                                                    key={v.path}
                                                    value={v.path}
                                                    onSelect={() => insertVariable(v.path)}
                                                >
                                                    <span className="font-mono text-xs">
                                                        {v.label}
                                                    </span>
                                                    {v.description && (
                                                        <span className="ml-auto text-xs text-muted-foreground truncate max-w-[100px]">
                                                            {v.description}
                                                        </span>
                                                    )}
                                                </CommandItem>
                                            ))}
                                        </CommandGroup>
                                    ))}
                                </CommandList>
                            </Command>
                        </PopoverContent>
                    </Popover>
                )}
            </div>
        </div>
    )
}
