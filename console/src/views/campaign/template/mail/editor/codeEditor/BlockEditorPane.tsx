import { FileText, LayoutGrid } from "lucide-react"
import type { User } from "@/types"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"
import { UserSelection } from "../../../UserSelection"
import { BlockEditor } from "./enterpriseHosts"
import type { BlockEditorTab, EditorMode, EmailDocument } from "./hooks/useEditorMode"
import { ModeToggle } from "./ModeControls"
import { SendTestButton } from "./PreviewToolbar"
import { TabButton } from "./TabButton"

interface BlockEditorPaneProps {
    projectId: string
    variableGroups: VariableGroup[]
    blocksData: EmailDocument | null
    onBlocksChange: (doc: EmailDocument, jsxSource: string) => void
    tab: BlockEditorTab
    onTabChange: (tab: BlockEditorTab) => void
    editorMode: EditorMode
    onModeSwitch: (mode: EditorMode) => void
    selectedUser: User | null
    onUserSelect: (user: User) => void
    sending: boolean
    onSendTest: () => void
}

/** The block editor (enterprise only) with its tab bar; it has no separate preview. */
export function BlockEditorPane({
    projectId,
    variableGroups,
    blocksData,
    onBlocksChange,
    tab,
    onTabChange,
    editorMode,
    onModeSwitch,
    selectedUser,
    onUserSelect,
    sending,
    onSendTest,
}: BlockEditorPaneProps) {
    return (
        <>
            <div className="flex items-center justify-between border-b bg-background">
                <div className="flex items-center">
                    <TabButton
                        active={tab === "editor"}
                        onClick={() => onTabChange("editor")}
                        icon={<LayoutGrid className="h-4 w-4" />}
                        label="Editor"
                    />
                    <TabButton
                        active={tab === "preview-text"}
                        onClick={() => onTabChange("preview-text")}
                        icon={<FileText className="h-4 w-4" />}
                        label="Preview Text"
                    />
                </div>
                <div className="flex items-center gap-1 pr-3">
                    <ModeToggle editorMode={editorMode} onModeSwitch={onModeSwitch} />
                    <div className="mx-1 h-4 w-px bg-border" />
                    <UserSelection
                        projectId={projectId}
                        value={selectedUser}
                        onChange={onUserSelect}
                        size="sm"
                        ariaLabel="Send test recipient"
                        searchInputAriaLabel="Search send test recipients"
                    />
                    <SendTestButton sending={sending} onClick={onSendTest} />
                </div>
            </div>

            <div className="flex-1 min-h-0">
                <BlockEditor
                    initialDocument={blocksData ?? undefined}
                    onChange={onBlocksChange}
                    activeTab={tab}
                    onTabChange={onTabChange}
                    variableGroups={variableGroups}
                />
            </div>
        </>
    )
}
