import { useCallback, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Editor, type OnMount } from "@monaco-editor/react"
import {
    ChevronDown,
    ChevronUp,
    AlertCircle,
    AlertTriangle,
    Braces,
    Cog,
    Undo2,
} from "lucide-react"

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/utils"
import type { User } from "@/types"
import { UserSelection } from "../../../UserSelection"

interface PropsEditorPanelProps {
    /** Current JSON string displayed in the editor */
    value: string
    /** Called with new JSON string whenever the user edits (every keystroke) */
    onChange: (value: string) => void
    /** Reset the editor content to the default props */
    onReset: () => void
    /** Whether the user has modified the props from the defaults */
    isCustomized: boolean
    /** Validation error message when JSON is invalid, null when valid */
    validationError: string | null
    /** Property paths in the edited props that are not part of the variable schema */
    extraProps: string[]
    /** Project ID for user search */
    projectId: string
    /** Currently selected user */
    selectedUser: User | null
    /** Called when a user is selected from the dropdown */
    onUserSelect: (user: User) => void
}

export function PropsEditorPanel({
    value,
    onChange,
    onReset,
    isCustomized,
    validationError,
    extraProps,
    projectId,
    selectedUser,
    onUserSelect,
}: PropsEditorPanelProps) {
    const { t } = useTranslation()
    const [collapsed, setCollapsed] = useState(false)
    const editorRef = useRef<Parameters<OnMount>[0] | null>(null)

    const handleEditorMount: OnMount = useCallback((editor) => {
        editorRef.current = editor
    }, [])

    const handleChange = useCallback(
        (val: string | undefined) => {
            onChange(val ?? "")
        },
        [onChange],
    )

    return (
        <div className={cn("flex flex-col border-t bg-background", collapsed ? "h-auto" : "h-70")}>
            {/* Header bar */}
            <div className="flex items-center justify-between border-b bg-background px-1">
                <button
                    onClick={() => setCollapsed((c) => !c)}
                    className={cn(
                        "flex items-center gap-2 px-3 py-2.5 text-sm font-medium transition-all duration-200 cursor-pointer shrink-0",
                        "border-b-2 -mb-px border-primary text-foreground",
                    )}
                >
                    <Braces className="h-4 w-4" />
                    <span>props.json</span>
                    {collapsed ? (
                        <ChevronUp className="h-3.5 w-3.5 ml-1 text-muted-foreground" />
                    ) : (
                        <ChevronDown className="h-3.5 w-3.5 ml-1 text-muted-foreground" />
                    )}
                </button>

                <div className="flex items-center gap-2 pr-2 min-w-0">
                    {!collapsed && validationError && (
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="flex items-center gap-1.5 text-destructive shrink-0">
                                    <AlertCircle className="h-3.5 w-3.5" />
                                    <span className="text-xs">
                                        {t(
                                            "campaign.template.email.editor.invalidJson",
                                            "Invalid JSON",
                                        )}
                                    </span>
                                </div>
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-xs">
                                {validationError}
                            </TooltipContent>
                        </Tooltip>
                    )}
                    <UserSelection
                        projectId={projectId}
                        value={selectedUser}
                        onChange={onUserSelect}
                        size="sm"
                    />
                </div>
            </div>

            {/* Collapsible editor content */}
            {!collapsed && (
                <div className="flex-1 min-h-0 relative">
                    {/* Warning banner for extra (non-schema) properties */}
                    {extraProps.length > 0 && (
                        <div className="flex items-start gap-2 px-3 py-2 bg-yellow-50 dark:bg-yellow-950/30 border-b border-yellow-200 dark:border-yellow-800 text-yellow-800 dark:text-yellow-200 text-xs">
                            <AlertTriangle className="h-3.5 w-3.5 mt-0.5 shrink-0" />
                            <span>
                                {t(
                                    "campaign.template.email.editor.unknownProps",
                                    "Unknown properties: ",
                                )}
                                <span className="font-mono">{extraProps.join(", ")}</span>
                            </span>
                        </div>
                    )}

                    <Editor
                        value={value}
                        defaultLanguage="json"
                        onChange={handleChange}
                        onMount={handleEditorMount}
                        options={{
                            automaticLayout: true,
                            minimap: { enabled: false },
                            fontSize: 12,
                            lineHeight: 18,
                            scrollBeyondLastLine: false,
                            padding: { top: 8, bottom: 8 },
                            renderLineHighlight: "line",
                            smoothScrolling: true,
                            cursorBlinking: "smooth",
                            cursorSmoothCaretAnimation: "on",
                            guides: { indentation: false },
                            stickyScroll: { enabled: false },
                            tabSize: 2,
                            wordWrap: "on",
                            folding: true,
                            lineNumbers: "on",
                            lineNumbersMinChars: 3,
                            glyphMargin: false,
                            formatOnPaste: true,
                            formatOnType: true,
                        }}
                    />

                    {/* Bottom-right status indicator — matches PlainTextEditor pattern */}
                    <div className="absolute bottom-3 right-3 z-10">
                        {isCustomized ? (
                            <button
                                onClick={onReset}
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
                                        "campaign.template.email.editor.revertToDefaults",
                                        "Revert to defaults",
                                    )}
                                </span>
                            </button>
                        ) : (
                            <div className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-muted/90 backdrop-blur-sm">
                                <Cog className="h-3 w-3 text-muted-foreground" />
                                <span className="text-xs text-muted-foreground font-medium">
                                    {t("campaign.template.email.editor.defaultProps", "Default")}
                                </span>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    )
}
