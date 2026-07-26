import {
    useCallback,
    useContext,
    useEffect,
    useMemo,
    useRef,
    useState,
    lazy,
    Suspense,
} from "react"
import { Editor } from "@monaco-editor/react"
import {
    Smartphone,
    Tablet,
    Monitor,
    FileCode,
    FileText,
    Eye,
    Rocket,
    Sparkles,
    Code2,
    Crosshair,
    LayoutGrid,
    AlertTriangle,
    Upload,
    Download,
} from "lucide-react"
import { toast } from "sonner"

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogFooter,
    DialogTitle,
    DialogDescription,
} from "@/components/ui/dialog"
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover"
import {
    DropdownMenu,
    DropdownMenuTrigger,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuLabel,
    DropdownMenuSub,
    DropdownMenuSubTrigger,
    DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
    Select,
    SelectTrigger,
    SelectContent,
    SelectItem,
    SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { ColorPicker } from "@/components/ui/color-picker"
import { TemplateInput } from "@/components/ui/template-input"
import { cn } from "@/utils"

import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import { useCampaignVariableContext } from "@/views/campaign/CampaignVariableContext"

import { PlainTextEditor, type PlainTextEditorRef } from "./PlainTextEditor"
import { EmailPreviewPanel } from "./EmailPreviewPanel"
import { PropsEditorPanel } from "./PropsEditorPanel"
import { MediaManager } from "@/components/media-manager"
import { EditorToolbar } from "./EditorToolbar"
import { TabButton, PreviewTab } from "./TabButton"
import { DEFAULT_REACT_EMAIL_TEMPLATE } from "./defaultTemplate"
import { templaticalPlainText, templaticalPreviewHtml } from "@/lib/templatical-preview"
import Iframe from "@/components/iframe"
import { Render } from "@/renderTemplates"
import { UserSelection } from "../../../UserSelection"

import type { Viewport, EditorTab } from "./types"
import { VIEWPORT_WIDTHS } from "./types"
import { isEnterprise } from "@/config/enterprise"
import type { EmailDocument, BlockEditorTab, BlockEditorHandle } from "./hooks/useEditorMode"
import {
    exportFileName,
    parseTemplaticalDocument,
    serializeTemplaticalDocument,
} from "../blockEditor/documentJson"

import {
    BuilderProvider,
    BuilderPanel,
    useBuilderActions,
    useBuilderActionsOptional,
    useBuilderThread,
    useBuilderStream,
    AIBuilderHostProvider,
} from "@lunogram-enterprise/ai-builder"

import Modal from "@/components/modal"
import { createUuid } from "@/utils"

/** All host UI components passed to BlockEditorHostProvider */
const hostComponents = {
    Button,
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    Popover,
    PopoverTrigger,
    PopoverContent,
    Tooltip,
    TooltipTrigger,
    TooltipContent,
    DropdownMenu,
    DropdownMenuTrigger,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuLabel,
    DropdownMenuSub,
    DropdownMenuSubTrigger,
    DropdownMenuSubContent,
    ScrollArea,
    Input,
    Label,
    Select,
    SelectTrigger,
    SelectContent,
    SelectItem,
    SelectValue,
    Separator,
    ColorPicker,
    MediaManager,
    TemplateInput,
} as const

const hostUtilities = { cn } as const

/** Host UI components for the AI builder package */
const aiBuilderHostComponents = {
    Badge,
    Button,
    Tooltip,
    TooltipTrigger,
    TooltipContent,
    ScrollArea,
    Modal,
} as const

const aiBuilderHostUtilities = { createUuid, cn } as const

const LazyBlockEditorWrapped = __ENTERPRISE__
    ? lazy(() =>
          import("@lunogram-enterprise/block-editor").then((mod) => ({
              default: function BlockEditorWrapped(props: {
                  initialDocument?: EmailDocument
                  onChange: (doc: EmailDocument, jsxSource: string) => void
                  activeTab: BlockEditorTab
                  onTabChange: (tab: BlockEditorTab) => void
                  variableGroups: {
                      label: string
                      variables: {
                          path: string
                          label: string
                          description?: string
                          types?: string[]
                          defaultValue?: unknown
                      }[]
                  }[]
              }) {
                  return (
                      <mod.BlockEditorHostProvider
                          components={hostComponents}
                          utilities={hostUtilities}
                          variableGroups={props.variableGroups}
                      >
                          <mod.BlockEditor
                              initialDocument={props.initialDocument}
                              onChange={props.onChange}
                              activeTab={props.activeTab}
                              onTabChange={props.onTabChange}
                          />
                      </mod.BlockEditorHostProvider>
                  )
              },
          })),
      )
    : lazy(() =>
          import("../blockEditor/TemplaticalBlockEditor").then((mod) => ({
              default: mod.TemplaticalBlockEditor,
          })),
      )

// Extracted hooks
import { useCompilation } from "./hooks/useCompilation"
import { usePreviewProps } from "../hooks/usePreviewProps"
import {
    useEditorMode,
    getInitialEditorMode,
    BLOCKS_MODE,
    type EditorMode,
} from "./hooks/useEditorMode"
import { useSendTestEmail } from "../hooks/useSendTestEmail"
import { useMonacoSetup } from "./hooks/useMonacoSetup"
import { useInsertActions } from "./hooks/useInsertActions"
import { useTemplatePersistence } from "../hooks/useTemplatePersistence"
import { compileEmail } from "./compileEmail"

export function CodeEditor() {
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)

    if (isEnterprise) {
        return (
            <AIBuilderHostProvider
                components={aiBuilderHostComponents}
                utilities={aiBuilderHostUtilities}
            >
                <BuilderProvider projectId={project.id} templateId={template.id}>
                    <CodeEditorInner />
                </BuilderProvider>
            </AIBuilderHostProvider>
        )
    }

    return <CodeEditorInner />
}

/**
 * Headless component that subscribes to builder stream/thread contexts
 * and runs the compile-check feedback loop when the AI finishes streaming.
 * Rendered as a child so CodeEditorInner itself doesn't re-render on
 * builder state changes.
 */
function BuilderCompileCheck({
    previewPropsRef,
}: {
    previewPropsRef: React.RefObject<Record<string, unknown>>
}) {
    const { reportCompileResult, startCompileCheck } = useBuilderActions()
    const { currentSource: builderCurrentSource } = useBuilderThread()
    const { isAgentTyping: builderIsAgentTyping } = useBuilderStream()

    const prevAgentTypingRef = useRef(builderIsAgentTyping)
    const compileCheckAbortRef = useRef<AbortController | null>(null)

    useEffect(() => {
        const wasTyping = prevAgentTypingRef.current
        prevAgentTypingRef.current = builderIsAgentTyping

        if (wasTyping && !builderIsAgentTyping && builderCurrentSource) {
            const source = builderCurrentSource

            compileCheckAbortRef.current?.abort()
            const controller = new AbortController()
            compileCheckAbortRef.current = controller

            startCompileCheck()

            void (async () => {
                try {
                    await compileEmail(source, previewPropsRef.current, controller.signal)
                    if (!controller.signal.aborted) {
                        reportCompileResult({ success: true })
                    }
                } catch (err) {
                    if (controller.signal.aborted) return
                    if (err instanceof DOMException && err.name === "AbortError") return

                    const errorMessage = err instanceof Error ? err.message : String(err)
                    console.error("[COMPILE_CHECK] compile error:", errorMessage)
                    reportCompileResult({ success: false, error: errorMessage })
                }
            })()
        }
    }, [
        builderIsAgentTyping,
        builderCurrentSource,
        previewPropsRef,
        reportCompileResult,
        startCompileCheck,
    ])

    useEffect(() => {
        return () => {
            compileCheckAbortRef.current?.abort()
        }
    }, [])

    return null
}

// ---------------------------------------------------------------------------
// Presentational sub-components
// ---------------------------------------------------------------------------

function ModeToggle({
    editorMode,
    onModeSwitch,
}: {
    editorMode: EditorMode
    onModeSwitch: (mode: EditorMode) => void
}) {
    return (
        <div className="flex items-center gap-1">
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={editorMode === BLOCKS_MODE ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => onModeSwitch(BLOCKS_MODE)}
                    >
                        <LayoutGrid className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Visual Editor</TooltipContent>
            </Tooltip>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={editorMode === "code" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => onModeSwitch("code")}
                    >
                        <Code2 className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Code Editor</TooltipContent>
            </Tooltip>
            {isEnterprise && (
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button
                            variant={editorMode === "builder" ? "secondary" : "ghost"}
                            size="sm"
                            className="h-8 w-8 p-0"
                            onClick={() => onModeSwitch("builder")}
                        >
                            <Sparkles className="h-4 w-4" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>AI Builder</TooltipContent>
                </Tooltip>
            )}
        </div>
    )
}

function ModeSwitchDialog({
    pendingMode,
    currentMode,
    onConfirm,
    onCancel,
}: {
    pendingMode: EditorMode | null
    currentMode: EditorMode
    onConfirm: () => void
    onCancel: () => void
}) {
    return (
        <Dialog open={pendingMode !== null} onOpenChange={(open) => !open && onCancel()}>
            <DialogContent showClose={false} className="max-w-md">
                <DialogHeader>
                    <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-amber-500/10">
                        <AlertTriangle className="h-5 w-5 text-amber-500" />
                    </div>
                    <DialogTitle className="text-center">Switch editor mode?</DialogTitle>
                    <DialogDescription className="text-center">
                        {currentMode === "blocks" ? (
                            <>
                                The block editor and code editor are not synchronized. Switching to
                                the code editor will remove your block layout. The generated code
                                will be preserved, but you won't be able to switch back with your
                                current layout.
                            </>
                        ) : (
                            <>
                                The block editor and code editor are not synchronized. Any code
                                changes you've made will not be reflected in the block editor. It
                                will start from its own saved state or a blank document.
                            </>
                        )}
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter className="sm:flex-row sm:gap-2">
                    <Button variant="outline" className="flex-1" onClick={onCancel}>
                        Cancel
                    </Button>
                    <Button className="flex-1" onClick={onConfirm}>
                        Switch editor
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

/**
 * Paste or upload a Templatical document to replace the current one.
 *
 * Import replaces the whole document rather than merging: the format has no
 * meaningful merge, and a partial import would leave the template in a state
 * neither the user nor the renderer could reason about.
 */
function ImportDocumentDialog({
    open,
    onOpenChange,
    value,
    onValueChange,
    error,
    fileInputRef,
    onFileSelected,
    onApply,
}: {
    open: boolean
    onOpenChange: (open: boolean) => void
    value: string
    onValueChange: (value: string) => void
    error: string | null
    fileInputRef: React.RefObject<HTMLInputElement | null>
    onFileSelected: (file: File) => void
    onApply: () => void
}) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-xl">
                <DialogHeader>
                    <DialogTitle>Import template JSON</DialogTitle>
                    <DialogDescription>
                        Paste a document exported from another template, or choose a{" "}
                        <code>.json</code> file. This replaces the current layout — it is not saved
                        until you save the template.
                    </DialogDescription>
                </DialogHeader>

                <Textarea
                    value={value}
                    onChange={(e) => onValueChange(e.target.value)}
                    placeholder='{ "blocks": [ … ], "settings": { … } }'
                    className="h-56 font-mono text-xs"
                    aria-label="Template JSON"
                />

                {error && (
                    <p className="flex items-start gap-1.5 text-sm text-destructive" role="alert">
                        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                        {error}
                    </p>
                )}

                <input
                    ref={fileInputRef}
                    type="file"
                    accept="application/json,.json"
                    className="hidden"
                    onChange={(e) => {
                        const file = e.target.files?.[0]
                        if (file) onFileSelected(file)
                        // Clear so picking the same file twice still fires.
                        e.target.value = ""
                    }}
                />

                <DialogFooter className="sm:flex-row sm:justify-between sm:gap-2">
                    <Button variant="outline" onClick={() => fileInputRef.current?.click()}>
                        Choose file…
                    </Button>
                    <div className="flex gap-2">
                        <Button variant="outline" onClick={() => onOpenChange(false)}>
                            Cancel
                        </Button>
                        <Button onClick={onApply} disabled={!value.trim()}>
                            Import
                        </Button>
                    </div>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function PreviewToolbar({
    viewport,
    setViewport,
    selectorActive,
    setSelectorActive,
    editorMode,
    sending,
    onSendTest,
}: {
    viewport: Viewport
    setViewport: (v: Viewport) => void
    selectorActive: boolean
    setSelectorActive: (v: boolean) => void
    editorMode: EditorMode
    sending: boolean
    onSendTest: () => void
}) {
    return (
        <div className="flex items-center gap-1 pr-3">
            {isEnterprise && editorMode === "builder" && (
                <>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                variant={selectorActive ? "secondary" : "ghost"}
                                size="sm"
                                className={`h-8 w-8 p-0 ${selectorActive ? "bg-blue-500/10 text-blue-600 hover:bg-blue-500/20 dark:text-blue-400" : ""}`}
                                onClick={() => setSelectorActive(!selectorActive)}
                            >
                                <Crosshair className="h-4 w-4" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                            {selectorActive ? "Cancel selection" : "Select element"}
                        </TooltipContent>
                    </Tooltip>
                    <div className="w-px h-4 bg-border mx-1" />
                </>
            )}
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={viewport === "mobile" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => setViewport("mobile")}
                    >
                        <Smartphone className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Mobile (375px)</TooltipContent>
            </Tooltip>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={viewport === "tablet" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => setViewport("tablet")}
                    >
                        <Tablet className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Tablet (768px)</TooltipContent>
            </Tooltip>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={viewport === "desktop" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => setViewport("desktop")}
                    >
                        <Monitor className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Desktop (1280px)</TooltipContent>
            </Tooltip>
            <div className="mx-1 h-4 w-px bg-border" />
            <Button
                variant="default"
                size="sm"
                className="h-8 gap-1.5 px-2.5"
                onClick={onSendTest}
                disabled={sending}
            >
                <Rocket className="h-3.5 w-3.5" />
                <span className="text-xs">Send test</span>
            </Button>
        </div>
    )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

function CodeEditorInner() {
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [template] = useContext(TemplateContext)
    const { variableGroups } = useCampaignVariableContext()
    const { selectSection, deselectSection } = useBuilderActionsOptional()

    const plainTextEditorRef = useRef<PlainTextEditorRef | null>(null)

    const initialEditorMode = getInitialEditorMode(template?.data)

    // Editor state — shared between code editor and builder. A template
    // authored visually has no JSX and must not be seeded with the starter
    // template: persistence would then store it as code.source and the
    // starter would resurface if the user ever switched to the code editor.
    const [code, setCode] = useState<string>(
        template?.data?.code?.source ??
            (initialEditorMode === BLOCKS_MODE ? "" : DEFAULT_REACT_EMAIL_TEMPLATE),
    )

    // Plain text state
    const [customPlainText, setCustomPlainText] = useState<string>(
        template?.data?.plaintext?.custom ?? "",
    )
    const [useCustomPlainText, setUseCustomPlainText] = useState<boolean>(
        !!template?.data?.plaintext?.custom,
    )

    // UI state
    const [viewport, setViewport] = useState<Viewport>("tablet")
    const [activeTab, setActiveTab] = useState<EditorTab>("code")

    // --- Extracted hooks ---
    const {
        previewProps,
        previewPropsRef,
        propsJsonString,
        propsValidationError,
        selectedUser,
        extraProps,
        handlePropsJsonChange,
        handlePropsReset,
        handleUserSelect,
        prevDefaultsRef,
    } = usePreviewProps(variableGroups)

    const { editorRef, handleEditorMount, handleCodeChange, insertAtCursorCode } = useMonacoSetup(
        variableGroups,
        setCode,
    )

    const {
        editorMode,
        pendingMode,
        selectorActive,
        setSelectorActive,
        blockEditorTab,
        setBlockEditorTab,
        blocksData,
        handleModeSwitch,
        confirmModeSwitch,
        cancelModeSwitch,
        handleBlocksChange,
    } = useEditorMode({
        initialMode: initialEditorMode,
        code,
        setCode,
        editorRef,
        initialBlocksData: (template?.data?.blocks as EmailDocument) ?? null,
    })

    const { compiledHtml, compileError, autoPlainText } = useCompilation(code, previewProps)

    // A visually authored template is rendered by the backend, which also
    // derives its plain text, so the value lands in the bundle at save time.
    // It therefore reflects the last save rather than unsaved edits.
    const blocksPlainText = useMemo(
        () => templaticalPlainText(template?.data?.code?.bundle),
        // @ts-expect-error data is a channel union; code exists on the email arm.
        [template?.data?.code?.bundle],
    )

    // The rendered email with the selected recipient's data substituted. The
    // HTML comes from the backend render and carries merge tags literally, so
    // resolving them here is the same Handlebars pass the other previews use.
    // With no user selected the tags stay visible, which is the honest default.
    const blocksPreviewHtml = useMemo(() => {
        const html = templaticalPreviewHtml(template?.data?.code?.bundle)
        if (!html || !selectedUser) return html
        return Render(html, { user: selectedUser })
        // @ts-expect-error data is a channel union; code exists on the email arm.
    }, [template?.data?.code?.bundle, selectedUser])

    // --- Document import/export -------------------------------------------
    // The document is portable JSON by design, so moving one between templates
    // needs no server support: export reads the copy already in `blocksData`,
    // and import pushes one into the mounted editor.
    const blockEditorHandleRef = useRef<BlockEditorHandle | null>(null)
    const [importOpen, setImportOpen] = useState(false)
    const [importText, setImportText] = useState("")
    const [importError, setImportError] = useState<string | null>(null)
    const importFileRef = useRef<HTMLInputElement>(null)

    const handleCopyDocument = useCallback(async () => {
        if (!blocksData) return
        try {
            await navigator.clipboard.writeText(serializeTemplaticalDocument(blocksData))
            toast.success("Template JSON copied to clipboard")
        } catch {
            toast.error("Couldn't copy to the clipboard")
        }
    }, [blocksData])

    const handleDownloadDocument = useCallback(() => {
        if (!blocksData) return
        const blob = new Blob([serializeTemplaticalDocument(blocksData)], {
            type: "application/json",
        })
        const url = URL.createObjectURL(blob)
        const link = document.createElement("a")
        link.href = url
        link.download = exportFileName(campaign.name, template.locale)
        link.click()
        URL.revokeObjectURL(url)
    }, [blocksData, campaign.name, template.locale])

    const handleImportFile = useCallback((file: File) => {
        void file
            .text()
            .then((text) => {
                setImportText(text)
                setImportError(null)
            })
            .catch(() => setImportError("Couldn't read that file."))
    }, [])

    const handleApplyImport = useCallback(() => {
        const result = parseTemplaticalDocument(importText)
        if (!result.ok) {
            setImportError(result.error)
            return
        }

        const handle = blockEditorHandleRef.current
        if (!handle) {
            setImportError("The editor is still loading — try again in a moment.")
            return
        }

        handle.setContent(result.doc)
        setImportOpen(false)
        setImportText("")
        setImportError(null)
        toast.success("Template imported. Save to keep it.")
    }, [importText])

    const { sending, handleSendTest } = useSendTestEmail({
        previewProps,
        projectId: project.id,
        campaignId: campaign.id,
        templateId: template.id,
    })

    const { imageModalOpen, setImageModalOpen, insertVariable, insertImage } = useInsertActions({
        activeTab,
        insertAtCursorCode,
        plainTextEditorRef,
    })

    useTemplatePersistence({
        editorMode,
        code,
        blocksData,
        autoPlainText,
        useCustomPlainText,
        customPlainText,
    })

    // Handle source changes from the AI builder
    const handleBuilderSourceChange = useCallback((source: string) => {
        setCode(source)
    }, [])

    // Handle section selection from the preview panel (builder mode only)
    const handleSectionSelect = useCallback(
        (section: { label: string; sectionId?: string; textContent?: string }) => {
            selectSection(section)
            setSelectorActive(false)
        },
        [selectSection, setSelectorActive],
    )

    const handleSectionDeselect = useCallback(
        (label: string) => {
            deselectSection(label)
        },
        [deselectSection],
    )

    // --- Block editor: full-width layout (no split panels, no preview) ---
    if (editorMode === "blocks") {
        return (
            <div className="flex flex-col h-full w-full">
                <div className="flex items-center justify-between border-b bg-background">
                    <div className="flex items-center">
                        <TabButton
                            active={blockEditorTab === "editor"}
                            onClick={() => setBlockEditorTab("editor")}
                            icon={<LayoutGrid className="h-4 w-4" />}
                            label="Editor"
                        />
                        {!isEnterprise && (
                            <TabButton
                                active={blockEditorTab === "preview"}
                                onClick={() => setBlockEditorTab("preview")}
                                icon={<Eye className="h-4 w-4" />}
                                label="Preview"
                            />
                        )}
                        <TabButton
                            active={blockEditorTab === "preview-text"}
                            onClick={() => setBlockEditorTab("preview-text")}
                            icon={<FileText className="h-4 w-4" />}
                            label="Preview Text"
                        />
                    </div>
                    <div className="flex items-center gap-1 pr-3">
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 w-8 p-0"
                                    onClick={() => {
                                        setImportText("")
                                        setImportError(null)
                                        setImportOpen(true)
                                    }}
                                >
                                    <Upload className="h-4 w-4" />
                                </Button>
                            </TooltipTrigger>
                            <TooltipContent>Import JSON</TooltipContent>
                        </Tooltip>
                        <DropdownMenu>
                            <Tooltip>
                                <TooltipTrigger asChild>
                                    <DropdownMenuTrigger asChild>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="h-8 w-8 p-0"
                                            disabled={!blocksData}
                                        >
                                            <Download className="h-4 w-4" />
                                        </Button>
                                    </DropdownMenuTrigger>
                                </TooltipTrigger>
                                <TooltipContent>Export JSON</TooltipContent>
                            </Tooltip>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem onSelect={() => void handleCopyDocument()}>
                                    Copy to clipboard
                                </DropdownMenuItem>
                                <DropdownMenuItem onSelect={handleDownloadDocument}>
                                    Download .json
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                        <div className="mx-1 h-4 w-px bg-border" />
                        <ModeToggle editorMode={editorMode} onModeSwitch={handleModeSwitch} />
                        <div className="mx-1 h-4 w-px bg-border" />
                        <UserSelection
                            projectId={project.id}
                            value={selectedUser}
                            onChange={handleUserSelect}
                            size="sm"
                            ariaLabel="Send test recipient"
                            searchInputAriaLabel="Search send test recipients"
                        />
                        <Button
                            variant="default"
                            size="sm"
                            className="h-8 gap-1.5 px-2.5"
                            onClick={handleSendTest}
                            disabled={sending}
                        >
                            <Rocket className="h-3.5 w-3.5" />
                            <span className="text-xs">Send test</span>
                        </Button>
                    </div>
                </div>

                <div className="flex-1 min-h-0">
                    {/* The enterprise block editor renders the plain-text tab
                        itself; Templatical does not, so the host swaps panels.
                        Keep the editor mounted underneath — remounting it on
                        every tab change would discard undo history. */}
                    <div className={cn("h-full", blockEditorTab !== "editor" && "hidden")}>
                        {LazyBlockEditorWrapped && (
                            <Suspense fallback={<div className="flex-1" />}>
                                <LazyBlockEditorWrapped
                                    initialDocument={blocksData ?? undefined}
                                    onChange={handleBlocksChange}
                                    activeTab={blockEditorTab}
                                    onTabChange={setBlockEditorTab}
                                    variableGroups={variableGroups}
                                    onReady={(handle: BlockEditorHandle | null) => {
                                        blockEditorHandleRef.current = handle
                                    }}
                                />
                            </Suspense>
                        )}
                    </div>
                    {!isEnterprise && blockEditorTab === "preview" && (
                        <div className="h-full w-full overflow-auto bg-muted/20 p-6">
                            <div className="mx-auto max-w-[700px] bg-white shadow-sm">
                                <Iframe content={blocksPreviewHtml} allowScroll={false} />
                            </div>
                        </div>
                    )}
                    {!isEnterprise && blockEditorTab === "preview-text" && (
                        <PlainTextEditor
                            ref={plainTextEditorRef}
                            autoText={blocksPlainText}
                            customText={customPlainText}
                            onCustomTextChange={setCustomPlainText}
                            useCustom={useCustomPlainText}
                            onToggleCustom={setUseCustomPlainText}
                            onImageClick={() => setImageModalOpen(true)}
                            onInsertVariable={insertVariable}
                            variableGroups={variableGroups}
                        />
                    )}
                </div>

                {isEnterprise && <BuilderCompileCheck previewPropsRef={previewPropsRef} />}

                <ModeSwitchDialog
                    pendingMode={pendingMode}
                    currentMode={editorMode}
                    onConfirm={confirmModeSwitch}
                    onCancel={cancelModeSwitch}
                />
                <MediaManager
                    open={imageModalOpen}
                    onOpenChange={setImageModalOpen}
                    onSelect={insertImage}
                />
                <ImportDocumentDialog
                    open={importOpen}
                    onOpenChange={setImportOpen}
                    value={importText}
                    onValueChange={(value) => {
                        setImportText(value)
                        setImportError(null)
                    }}
                    error={importError}
                    fileInputRef={importFileRef}
                    onFileSelected={handleImportFile}
                    onApply={handleApplyImport}
                />
            </div>
        )
    }

    // --- Code editor / AI builder: split-panel layout with preview ---
    return (
        <div className="flex flex-col h-full w-full">
            <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0">
                {/* Left panel: Code editor or Builder chat */}
                <ResizablePanel defaultSize={50} minSize={30} className="overflow-hidden">
                    <div className="flex flex-col h-full">
                        <div className="flex items-center justify-between border-b bg-background">
                            <div className="flex items-center">
                                {editorMode === "code" ? (
                                    <>
                                        <TabButton
                                            active={activeTab === "code"}
                                            onClick={() => setActiveTab("code")}
                                            icon={<FileCode className="h-4 w-4" />}
                                            label="template.tsx"
                                        />
                                        <TabButton
                                            active={activeTab === "plaintext"}
                                            onClick={() => setActiveTab("plaintext")}
                                            icon={<FileText className="h-4 w-4" />}
                                            label="plaintext.txt"
                                        />
                                    </>
                                ) : (
                                    <TabButton
                                        active={true}
                                        onClick={() => {}}
                                        icon={<Sparkles className="h-4 w-4" />}
                                        label="AI Builder"
                                    />
                                )}
                            </div>
                            <div className="pr-3">
                                <ModeToggle
                                    editorMode={editorMode}
                                    onModeSwitch={handleModeSwitch}
                                />
                            </div>
                        </div>

                        <div className="flex-1 min-h-0 relative">
                            {editorMode === "code" ? (
                                <>
                                    {activeTab === "code" ? (
                                        <>
                                            <EditorToolbar
                                                onImageClick={() => setImageModalOpen(true)}
                                                onInsertVariable={insertVariable}
                                                variableGroups={variableGroups}
                                                showImageButton={true}
                                            />
                                            <Editor
                                                value={code}
                                                defaultLanguage="typescript"
                                                defaultPath="file:///template.tsx"
                                                onChange={handleCodeChange}
                                                onMount={handleEditorMount}
                                                options={{
                                                    automaticLayout: true,
                                                    minimap: { enabled: false },
                                                    fontSize: 13,
                                                    lineHeight: 20,
                                                    scrollBeyondLastLine: false,
                                                    padding: { top: 12, bottom: 12 },
                                                    renderLineHighlight: "line",
                                                    smoothScrolling: true,
                                                    cursorBlinking: "smooth",
                                                    cursorSmoothCaretAnimation: "on",
                                                    bracketPairColorization: { enabled: true },
                                                    guides: { indentation: false },
                                                    stickyScroll: { enabled: false },
                                                    tabSize: 2,
                                                    wordWrap: "on",
                                                    folding: true,
                                                    suggest: {
                                                        showKeywords: true,
                                                        showSnippets: true,
                                                    },
                                                }}
                                            />
                                        </>
                                    ) : (
                                        <PlainTextEditor
                                            ref={plainTextEditorRef}
                                            autoText={autoPlainText}
                                            customText={customPlainText}
                                            onCustomTextChange={setCustomPlainText}
                                            useCustom={useCustomPlainText}
                                            onToggleCustom={setUseCustomPlainText}
                                            onImageClick={() => setImageModalOpen(true)}
                                            onInsertVariable={insertVariable}
                                            variableGroups={variableGroups}
                                        />
                                    )}
                                </>
                            ) : isEnterprise ? (
                                <BuilderPanel
                                    currentSource={code}
                                    onSourceChange={handleBuilderSourceChange}
                                />
                            ) : null}
                        </div>
                    </div>
                </ResizablePanel>

                <ResizableHandle withHandle />

                {/* Right panel: Preview + Props editor */}
                <ResizablePanel defaultSize={50} minSize={25} className="overflow-hidden">
                    <div className="flex flex-col h-full">
                        <div className="flex items-center justify-between border-b bg-background">
                            <PreviewTab icon={<Eye className="h-4 w-4" />} label="Preview" />
                            <PreviewToolbar
                                viewport={viewport}
                                setViewport={setViewport}
                                selectorActive={selectorActive}
                                setSelectorActive={(v) => setSelectorActive(v)}
                                editorMode={editorMode}
                                sending={sending}
                                onSendTest={handleSendTest}
                            />
                        </div>

                        <div className="flex-1 min-h-0 min-w-0 overflow-hidden bg-muted/30">
                            <EmailPreviewPanel
                                html={compiledHtml}
                                error={compileError}
                                viewport={viewport}
                                viewportWidth={VIEWPORT_WIDTHS[viewport]}
                                onSectionSelect={handleSectionSelect}
                                onSectionDeselect={handleSectionDeselect}
                                selectorActive={selectorActive}
                            />
                        </div>

                        <PropsEditorPanel
                            value={propsJsonString}
                            onChange={handlePropsJsonChange}
                            onReset={handlePropsReset}
                            isCustomized={propsJsonString !== prevDefaultsRef.current}
                            validationError={propsValidationError}
                            extraProps={extraProps}
                            projectId={project.id}
                            selectedUser={selectedUser}
                            onUserSelect={handleUserSelect}
                        />
                    </div>
                </ResizablePanel>
            </ResizablePanelGroup>

            {isEnterprise && <BuilderCompileCheck previewPropsRef={previewPropsRef} />}

            <ModeSwitchDialog
                pendingMode={pendingMode}
                currentMode={editorMode}
                onConfirm={confirmModeSwitch}
                onCancel={cancelModeSwitch}
            />
            <MediaManager
                open={imageModalOpen}
                onOpenChange={setImageModalOpen}
                onSelect={insertImage}
            />
        </div>
    )
}
