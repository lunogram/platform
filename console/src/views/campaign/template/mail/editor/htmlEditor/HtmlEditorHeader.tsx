import { useState, useEffect, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Code2, Undo2, Redo2, Braces, ChevronDown, User, Link, Mail, Hash } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
    DropdownMenuSub,
    DropdownMenuSubTrigger,
    DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import type { editor } from "monaco-editor"
import type CodeStore from "../codeEditorPlugins/CodeStore"

interface TemplateVariable {
    label: string
    value: string
    description?: string
}

interface TemplateVariableGroup {
    label: string
    icon: React.ReactNode
    variables: TemplateVariable[]
}

const templateVariableGroups: TemplateVariableGroup[] = [
    {
        label: "User",
        icon: <User className="h-4 w-4" />,
        variables: [
            { label: "Email", value: "{{user.email}}", description: "User's email address" },
            {
                label: "External ID",
                value: "{{user.external_id}}",
                description: "User's external identifier",
            },
            { label: "First Name", value: "{{user.first_name}}", description: "User's first name" },
            { label: "Last Name", value: "{{user.last_name}}", description: "User's last name" },
            { label: "Full Name", value: "{{user.name}}", description: "User's full name" },
            { label: "Phone", value: "{{user.phone}}", description: "User's phone number" },
            { label: "Timezone", value: "{{user.timezone}}", description: "User's timezone" },
            { label: "Locale", value: "{{user.locale}}", description: "User's locale preference" },
        ],
    },
    {
        label: "Links",
        icon: <Link className="h-4 w-4" />,
        variables: [
            {
                label: "Unsubscribe URL",
                value: "{{unsubscribe_url}}",
                description: "Link to unsubscribe",
            },
            {
                label: "Preferences URL",
                value: "{{preferences_url}}",
                description: "Link to email preferences",
            },
            {
                label: "Web Version URL",
                value: "{{web_version_url}}",
                description: "View in browser link",
            },
        ],
    },
    {
        label: "Campaign",
        icon: <Mail className="h-4 w-4" />,
        variables: [
            {
                label: "Campaign Name",
                value: "{{campaign.name}}",
                description: "Name of the campaign",
            },
            { label: "Subject", value: "{{campaign.subject}}", description: "Email subject line" },
        ],
    },
    {
        label: "Other",
        icon: <Hash className="h-4 w-4" />,
        variables: [
            { label: "Current Date", value: "{{now | date}}", description: "Current date" },
            { label: "Current Year", value: "{{now | date: '%Y'}}", description: "Current year" },
        ],
    },
]

interface HtmlEditorHeaderProps {
    editorRef: React.MutableRefObject<editor.IStandaloneCodeEditor | null>
    codeStore: typeof CodeStore
}

export function HtmlEditorHeader({ editorRef }: HtmlEditorHeaderProps) {
    const { t } = useTranslation()
    const [, setCanUndo] = useState(false)
    const [, setCanRedo] = useState(false)

    // Update undo/redo state when editor content changes
    const updateUndoRedoState = useCallback(() => {
        const editor = editorRef.current
        if (editor) {
            const model = editor.getModel()
            if (model) {
                // Monaco doesn't expose canUndo/canRedo directly, so we track state after operations
                // This is a simplified approach - in production you might want to listen to model changes
                setCanUndo(true) // Will be refined with actual editor state
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

    const handleFormat = () => {
        const editor = editorRef.current
        if (editor) {
            editor.getAction("editor.action.formatDocument")?.run()
            editor.focus()
        }
    }

    const insertVariable = (variable: string) => {
        const editor = editorRef.current
        if (editor) {
            const selection = editor.getSelection()
            if (selection) {
                editor.executeEdits("insert-variable", [
                    {
                        range: selection,
                        text: variable,
                        forceMoveMarkers: true,
                    },
                ])
                editor.focus()
            }
        }
    }

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

                {/* Divider */}
                <div className="h-5 w-px bg-border shrink-0" />

                {/* Format Button */}
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button variant="ghost" size="sm" onClick={handleFormat}>
                            <Braces className="h-4 w-4" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>
                        {t("campaign.template.email.editor.formatCode", "Format HTML")}
                    </TooltipContent>
                </Tooltip>

                {/* Template Variables Dropdown */}
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="sm" className="gap-1.5">
                            <Hash className="h-4 w-4" />
                            <ChevronDown className="h-3 w-3 opacity-50" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start" className="w-48">
                        {templateVariableGroups.map((group, groupIndex) => (
                            <div key={group.label}>
                                {groupIndex > 0 && <DropdownMenuSeparator />}
                                <DropdownMenuSub>
                                    <DropdownMenuSubTrigger className="gap-2">
                                        {group.icon}
                                        {group.label}
                                    </DropdownMenuSubTrigger>
                                    <DropdownMenuSubContent className="w-56">
                                        {group.variables.map((variable) => (
                                            <DropdownMenuItem
                                                key={variable.value}
                                                onClick={() => insertVariable(variable.value)}
                                                className="flex flex-col items-start gap-1 py-2"
                                            >
                                                <span className="font-medium">
                                                    {variable.label}
                                                </span>
                                                <span className="text-xs text-muted-foreground font-mono">
                                                    {variable.value}
                                                </span>
                                            </DropdownMenuItem>
                                        ))}
                                    </DropdownMenuSubContent>
                                </DropdownMenuSub>
                            </div>
                        ))}
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>
        </div>
    )
}
