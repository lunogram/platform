import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Braces, ImageIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

interface EditorToolbarProps {
    onImageClick: () => void
    onInsertVariable: (path: string) => void
    variableGroups: VariableGroup[]
    /** Whether to show the image button (only for code editor) */
    showImageButton?: boolean
}

/**
 * Floating toolbar for the email editor with image and variable insertion buttons.
 * Designed to be positioned absolutely in the top-right corner of the editor area.
 */
export function EditorToolbar({
    onImageClick,
    onInsertVariable,
    variableGroups,
    showImageButton = true,
}: EditorToolbarProps) {
    const { t } = useTranslation()
    const [variablePickerOpen, setVariablePickerOpen] = useState(false)

    const hasVariables = variableGroups.some((g) => g.variables.length > 0)

    const handleInsertVariable = (path: string) => {
        onInsertVariable(path)
        setVariablePickerOpen(false)
    }

    return (
        <div className="absolute top-2 right-4 z-10 flex items-center gap-1 bg-background/80 backdrop-blur-sm rounded-md border shadow-sm p-0.5">
            {showImageButton && (
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 p-0"
                            onClick={onImageClick}
                        >
                            <ImageIcon className="h-3.5 w-3.5" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>
                        {t("campaign.template.email.editor.images", "Images")}
                    </TooltipContent>
                </Tooltip>
            )}

            {hasVariables && (
                <Popover open={variablePickerOpen} onOpenChange={setVariablePickerOpen}>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <PopoverTrigger asChild>
                                <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
                                    <Braces className="h-3.5 w-3.5" />
                                </Button>
                            </PopoverTrigger>
                        </TooltipTrigger>
                        <TooltipContent>
                            {t("campaign.template.email.editor.variables", "Variables")}
                        </TooltipContent>
                    </Tooltip>
                    <PopoverContent
                        className="w-72 p-0"
                        align="end"
                        side="bottom"
                        onOpenAutoFocus={(e) => e.preventDefault()}
                    >
                        <Command>
                            <CommandInput
                                placeholder={t(
                                    "campaign.template.email.editor.searchVariables",
                                    "Search variables...",
                                )}
                            />
                            <CommandList>
                                <CommandEmpty>
                                    {t(
                                        "campaign.template.email.editor.noVariables",
                                        "No variables found.",
                                    )}
                                </CommandEmpty>
                                {variableGroups.map((group) => (
                                    <CommandGroup key={group.label} heading={group.label}>
                                        {group.variables.map((v) => (
                                            <CommandItem
                                                key={v.path}
                                                value={v.path}
                                                onSelect={() => handleInsertVariable(v.path)}
                                            >
                                                <span className="font-mono text-xs">{v.label}</span>
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
    )
}
