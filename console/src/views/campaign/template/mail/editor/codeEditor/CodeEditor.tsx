import { useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import { Editor, type OnMount } from "@monaco-editor/react"
import type { editor } from "monaco-editor"
import { Smartphone, Tablet, Monitor, FileCode, FileText, Eye, Rocket } from "lucide-react"
import { toast } from "sonner"

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

import { TemplateWorkflowContext } from "../../../contexts"
import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import { useCampaignVariableContext } from "@/views/campaign/CampaignVariableContext"
import api from "@/api"

import { PlainTextEditor, type PlainTextEditorRef } from "./PlainTextEditor"
import { EmailPreviewPanel } from "./EmailPreviewPanel"
import { PropsEditorPanel } from "./PropsEditorPanel"
import { ImageLibraryModal } from "./ImageLibraryModal"
import { EditorToolbar } from "./EditorToolbar"
import { TabButton, PreviewTab } from "./TabButton"
import { DEFAULT_REACT_EMAIL_TEMPLATE } from "./defaultTemplate"

import type { User } from "@/types"
import type { Viewport, EditorTab } from "./types"
import { VIEWPORT_WIDTHS } from "./types"
import {
    buildPreviewProps,
    buildSchemaPaths,
    findExtraProps,
    mergeUserIntoProps,
    generatePropsTypeDeclarations,
} from "./variableScope"
import { configureMonaco, updatePropsTypeDeclarations } from "./monacoSetup"
import { compileEmail } from "./compileEmail"

export function CodeEditor() {
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [template, setTemplate] = useContext(TemplateContext)

    const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null)
    const monacoRef = useRef<Parameters<OnMount>[1] | null>(null)
    const plainTextEditorRef = useRef<PlainTextEditorRef | null>(null)
    const { variableGroups } = useCampaignVariableContext()

    // Editor state
    const [code, setCode] = useState<string>(
        template?.data?.code?.source ?? DEFAULT_REACT_EMAIL_TEMPLATE,
    )
    const [compiledHtml, setCompiledHtml] = useState<string>("")
    const [compileError, setCompileError] = useState<string | null>(null)

    // Plain text state
    const [autoPlainText, setAutoPlainText] = useState<string>("")
    const [customPlainText, setCustomPlainText] = useState<string>(
        template?.data?.plaintext?.custom ?? "",
    )
    const [useCustomPlainText, setUseCustomPlainText] = useState<boolean>(
        !!template?.data?.plaintext?.custom,
    )

    // UI state
    const [viewport, setViewport] = useState<Viewport>("tablet")
    const [imageModalOpen, setImageModalOpen] = useState(false)
    const [activeTab, setActiveTab] = useState<EditorTab>("code")

    // Build default preview props from variable groups
    const defaultPreviewProps = useMemo(() => buildPreviewProps(variableGroups), [variableGroups])

    // Editable JSON string for the props editor panel
    const [propsJsonString, setPropsJsonString] = useState<string>(() =>
        JSON.stringify(defaultPreviewProps, null, 2),
    )
    const [propsValidationError, setPropsValidationError] = useState<string | null>(null)

    // The actual preview props used for compilation — only updated when JSON is valid
    const [previewProps, setPreviewProps] = useState<Record<string, unknown>>(defaultPreviewProps)

    // Selected user for populating user.* props with real data
    const [selectedUser, setSelectedUser] = useState<User | null>(null)

    // Send test email state
    const [sending, setSending] = useState(false)

    // When variable groups change (e.g. new variables added), regenerate defaults
    // but only if the user hasn't customised the props (still matches old defaults)
    const prevDefaultsRef = useRef<string>(JSON.stringify(defaultPreviewProps, null, 2))
    useEffect(() => {
        const newDefaults = JSON.stringify(defaultPreviewProps, null, 2)
        if (prevDefaultsRef.current !== newDefaults) {
            // If the current editor content matches the old defaults, update to new defaults
            if (propsJsonString === prevDefaultsRef.current) {
                setPropsJsonString(newDefaults)
                setPreviewProps(defaultPreviewProps)
                setPropsValidationError(null)
            }
            prevDefaultsRef.current = newDefaults
        }
    }, [defaultPreviewProps, propsJsonString])

    // Handle props JSON editing — validate and update preview props when valid
    const handlePropsJsonChange = useCallback((value: string) => {
        setPropsJsonString(value)
        try {
            const parsed = JSON.parse(value) as Record<string, unknown>
            setPreviewProps(parsed)
            setPropsValidationError(null)
        } catch (err) {
            setPropsValidationError(err instanceof Error ? err.message : String(err))
        }
    }, [])

    // Reset props editor to defaults and clear selected user
    const handlePropsReset = useCallback(() => {
        const json = JSON.stringify(defaultPreviewProps, null, 2)
        setPropsJsonString(json)
        setPreviewProps(defaultPreviewProps)
        setPropsValidationError(null)
        setSelectedUser(null)
    }, [defaultPreviewProps])

    // Handle user selection — merge real user data into props
    const handleUserSelect = useCallback(
        (user: User) => {
            setSelectedUser(user)
            const merged = mergeUserIntoProps(defaultPreviewProps, user)
            const json = JSON.stringify(merged, null, 2)
            setPropsJsonString(json)
            setPreviewProps(merged)
            setPropsValidationError(null)
        },
        [defaultPreviewProps],
    )

    // Send a test email using the current preview props
    const handleSendTest = useCallback(async () => {
        const userProps = previewProps.user as Record<string, unknown> | undefined
        const email = userProps?.email
        if (typeof email !== "string" || !email) {
            toast.error("No recipient email found. Set user.email in the props panel below.")
            return
        }

        setSending(true)
        try {
            await api.campaigns.templates.sendTest(project.id, campaign.id, template.id, {
                to: email,
                props: previewProps,
            })
            toast.success(`Test email sent to ${email}`)
        } catch {
            toast.error("Failed to send test email")
        } finally {
            setSending(false)
        }
    }, [previewProps, project.id, campaign.id, template.id])

    // Detect extra properties not in the variable schema
    const schemaPaths = useMemo(() => buildSchemaPaths(variableGroups), [variableGroups])
    const extraProps = useMemo(
        () => (propsValidationError ? [] : findExtraProps(previewProps, schemaPaths)),
        [previewProps, schemaPaths, propsValidationError],
    )

    // Generate TypeScript type declarations for IntelliSense and
    // update Monaco whenever variable groups change
    const propsTypeDeclarations = useMemo(
        () => generatePropsTypeDeclarations(variableGroups),
        [variableGroups],
    )

    useEffect(() => {
        const monaco = monacoRef.current
        if (!monaco) return
        updatePropsTypeDeclarations(monaco, propsTypeDeclarations)
    }, [propsTypeDeclarations])

    const compileTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
    const compileAbortRef = useRef<AbortController | null>(null)

    const runCompile = useCallback(async (source: string, props: Record<string, unknown>) => {
        // Cancel any in-flight compilation
        if (compileAbortRef.current) {
            compileAbortRef.current.abort()
        }
        const abortController = new AbortController()
        compileAbortRef.current = abortController

        try {
            const result = await compileEmail(source, props, abortController.signal)

            // Check if aborted
            if (abortController.signal.aborted) return

            setCompiledHtml(result.html)
            setCompileError(null)
            setAutoPlainText(result.plainText)
        } catch (err) {
            // Ignore abort errors
            if (abortController.signal.aborted) return
            if (err instanceof DOMException && err.name === "AbortError") return

            console.error("[CodeEditor] Compilation error:", err)
            setCompileError(err instanceof Error ? err.message : String(err))
        }
    }, [])

    // Debounced compile on code change (also re-compile when preview props change)
    useEffect(() => {
        if (compileTimeoutRef.current) {
            clearTimeout(compileTimeoutRef.current)
        }
        compileTimeoutRef.current = setTimeout(() => {
            void runCompile(code, previewProps)
        }, 600)

        return () => {
            if (compileTimeoutRef.current) {
                clearTimeout(compileTimeoutRef.current)
            }
            // Abort any in-flight compilation on cleanup
            if (compileAbortRef.current) {
                compileAbortRef.current.abort()
            }
        }
    }, [code, previewProps, runCompile])

    onSubmit(async () => {
        const updated = await api.campaigns.templates.update(project.id, campaign.id, template.id, {
            data: {
                ...template.data,
                type: "react-email",
                code: {
                    source: code,
                },
                plaintext: {
                    generated: autoPlainText,
                    ...(useCustomPlainText && customPlainText ? { custom: customPlainText } : {}),
                },
            },
        })

        setTemplate(updated)
        return true
    })

    const handleEditorMount: OnMount = useCallback(
        (editor, monaco) => {
            editorRef.current = editor
            monacoRef.current = monaco
            configureMonaco(editor, monaco, propsTypeDeclarations)
        },
        [propsTypeDeclarations],
    )

    // Handle code changes
    const handleCodeChange = useCallback((value: string | undefined) => {
        setCode(value ?? "")
    }, [])

    // Insert text at cursor position in the code editor (Monaco)
    const insertAtCursorCode = useCallback((text: string) => {
        const editor = editorRef.current
        if (!editor) return

        const selection = editor.getSelection()
        if (selection) {
            editor.executeEdits("insert", [
                {
                    range: selection,
                    text,
                    forceMoveMarkers: true,
                },
            ])
            editor.focus()
        }
    }, [])

    // Insert text at cursor position - routes to the correct editor based on active tab
    const insertAtCursor = useCallback(
        (text: string) => {
            if (activeTab === "code") {
                insertAtCursorCode(text)
            } else {
                plainTextEditorRef.current?.insertAtCursor(text)
            }
        },
        [activeTab, insertAtCursorCode],
    )

    // Insert variable at cursor — different format for code vs plain text
    const insertVariable = useCallback(
        (path: string) => {
            if (activeTab === "plaintext") {
                // Plain text keeps handlebars syntax
                plainTextEditorRef.current?.insertAtCursor(`{{ ${path} }}`)
                return
            }

            // Code editor: insert as JSX expression using props access
            const cleanPath = path.split(" ")[0] // handle "now | date"
            const expression = `{props.${cleanPath}}`

            insertAtCursorCode(expression)
        },
        [activeTab, insertAtCursorCode],
    )

    // Insert image at cursor (format depends on editor type)
    const insertImage = useCallback(
        (url: string, alt: string) => {
            if (activeTab === "code") {
                // In code editor, insert JSX component
                insertAtCursor(`<Img src="${url}" alt="${alt}" width="600" />`)
            } else {
                // In plain text, just insert the URL (images don't render in plain text)
                insertAtCursor(url)
            }
            setImageModalOpen(false)
        },
        [activeTab, insertAtCursor],
    )

    return (
        <div className="flex flex-col h-full w-full">
            <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0">
                {/* Left panel: Code editor */}
                <ResizablePanel defaultSize={50} minSize={30} className="overflow-hidden">
                    <div className="flex flex-col h-full">
                        {/* Editor tab bar */}
                        <div className="flex items-center border-b bg-background">
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
                        </div>

                        {/* Editor content */}
                        <div className="flex-1 min-h-0 relative">
                            {activeTab === "code" ? (
                                <>
                                    <EditorToolbar
                                        onImageClick={() => setImageModalOpen(true)}
                                        onInsertVariable={insertVariable}
                                        variableGroups={[]}
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
                        </div>
                    </div>
                </ResizablePanel>

                <ResizableHandle withHandle />

                {/* Right panel: Preview + Props editor */}
                <ResizablePanel defaultSize={50} minSize={25} className="overflow-hidden">
                    <div className="flex flex-col h-full">
                        {/* Preview tab bar - consistent styling with editor tabs */}
                        <div className="flex items-center justify-between border-b bg-background">
                            <PreviewTab icon={<Eye className="h-4 w-4" />} label="Preview" />
                            <div className="flex items-center gap-1 pr-3">
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
                                    onClick={handleSendTest}
                                    disabled={sending}
                                >
                                    <Rocket className="h-3.5 w-3.5" />
                                    <span className="text-xs">Send test</span>
                                </Button>
                            </div>
                        </div>

                        {/* Preview content — takes remaining space above props editor */}
                        <div className="flex-1 min-h-0 min-w-0 overflow-hidden bg-muted/30">
                            <EmailPreviewPanel
                                html={compiledHtml}
                                error={compileError}
                                viewport={viewport}
                                viewportWidth={VIEWPORT_WIDTHS[viewport]}
                            />
                        </div>

                        {/* Props editor panel — collapsible, pinned to bottom */}
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

            {/* Modals */}
            <ImageLibraryModal
                open={imageModalOpen}
                onOpenChange={setImageModalOpen}
                onInsert={insertImage}
            />
        </div>
    )
}
