import { useCallback, useContext, useRef, useState } from "react"
import { Editor } from "@monaco-editor/react"
import { FileCode, FileText, Eye, Sparkles } from "lucide-react"

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable"
import { MediaManager } from "@/components/media-manager"
import { isEnterprise } from "@/config/enterprise"
import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import { useCampaignVariableContext } from "@/views/campaign/CampaignVariableContext"
import { BuilderPanel, useBuilderActionsOptional } from "@lunogram-enterprise/ai-builder"

import { StepLayout } from "../../../StepLayout"
import { usePreviewProps } from "../hooks/usePreviewProps"
import { useSendTestEmail } from "../hooks/useSendTestEmail"
import { useEmailTemplateSave } from "../hooks/useEmailTemplateSave"
import { BlockEditorPane } from "./BlockEditorPane"
import { BuilderCompileCheck } from "./BuilderCompileCheck"
import { DEFAULT_REACT_EMAIL_TEMPLATE } from "./defaultTemplate"
import { EditorToolbar } from "./EditorToolbar"
import { SOURCE_EDITOR_OPTIONS } from "./editorOptions"
import { EmailPreviewPanel } from "./EmailPreviewPanel"
import { BuilderHost } from "./enterpriseHosts"
import { ModeSwitchDialog, ModeToggle } from "./ModeControls"
import { PlainTextEditor, type PlainTextEditorRef } from "./PlainTextEditor"
import { PreviewToolbar } from "./PreviewToolbar"
import { PropsEditorPanel } from "./PropsEditorPanel"
import { TabButton, PreviewTab } from "./TabButton"
import { useCompilation } from "./hooks/useCompilation"
import { useEditorMode, getInitialEditorMode } from "./hooks/useEditorMode"
import { useInsertActions } from "./hooks/useInsertActions"
import { useMonacoSetup } from "./hooks/useMonacoSetup"
import type { Viewport, EditorTab } from "./types"
import { VIEWPORT_WIDTHS } from "./types"

export function CodeEditor() {
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)

    if (isEnterprise) {
        return (
            <BuilderHost projectId={project.id} templateId={template.id}>
                <CodeEditorInner />
            </BuilderHost>
        )
    }

    return <CodeEditorInner />
}

function CodeEditorInner() {
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [template] = useContext(TemplateContext)
    const { variableGroups } = useCampaignVariableContext()
    const { selectSection, deselectSection } = useBuilderActionsOptional()

    // The editor step only exists for email campaigns.
    const email = template.type === "email" ? template.data : undefined

    const plainTextEditorRef = useRef<PlainTextEditorRef | null>(null)

    // Editor state — shared between code editor and builder
    const [code, setCode] = useState<string>(email?.code?.source ?? DEFAULT_REACT_EMAIL_TEMPLATE)

    // Plain text state
    const [customPlainText, setCustomPlainText] = useState<string>(email?.plaintext?.custom ?? "")
    const [useCustomPlainText, setUseCustomPlainText] = useState<boolean>(
        !!email?.plaintext?.custom,
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
        initialMode: getInitialEditorMode(email),
        code,
        setCode,
        editorRef,
        initialBlocksData: email?.blocks ?? null,
    })

    const { compiledHtml, compileError, autoPlainText } = useCompilation(code, previewProps)

    const save = useEmailTemplateSave({
        editorMode,
        code,
        blocksData,
        useCustomPlainText,
        customPlainText,
    })

    const { sending, handleSendTest } = useSendTestEmail({
        save,
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
            <StepLayout save={save}>
                <div className="flex flex-col h-full w-full">
                    <BlockEditorPane
                        projectId={project.id}
                        variableGroups={variableGroups}
                        blocksData={blocksData}
                        onBlocksChange={handleBlocksChange}
                        tab={blockEditorTab}
                        onTabChange={setBlockEditorTab}
                        editorMode={editorMode}
                        onModeSwitch={handleModeSwitch}
                        selectedUser={selectedUser}
                        onUserSelect={handleUserSelect}
                        sending={sending}
                        onSendTest={handleSendTest}
                    />

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
            </StepLayout>
        )
    }

    // --- Code editor / AI builder: split-panel layout with preview ---
    return (
        <StepLayout save={save}>
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
                                                    options={SOURCE_EDITOR_OPTIONS}
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
        </StepLayout>
    )
}
