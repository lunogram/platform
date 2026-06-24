import { useCallback, useRef, useState } from "react"
import type { editor } from "monaco-editor"
import { isEnterprise } from "@/config/enterprise"

/**
 * Minimal local type aliases for the block editor data model.
 * The full types live in @lunogram-enterprise/block-editor (enterprise only).
 * These are kept intentionally opaque — the hooks only pass them around.
 */
export type EmailDocument = Record<string, unknown>
export type BlockEditorTab = "editor" | "preview-text"

/** The top-level editing mode: code editor, AI builder, or block editor */
export type EditorMode = "code" | "builder" | "blocks"

interface UseEditorModeOptions {
    initialMode: EditorMode
    code: string
    setCode: (code: string) => void
    editorRef: React.RefObject<editor.IStandaloneCodeEditor | null>
    initialBlocksData: EmailDocument | null
}

export interface UseEditorModeResult {
    editorMode: EditorMode
    pendingMode: EditorMode | null
    selectorActive: boolean
    setSelectorActive: React.Dispatch<React.SetStateAction<boolean>>
    blockEditorTab: BlockEditorTab
    setBlockEditorTab: React.Dispatch<React.SetStateAction<BlockEditorTab>>
    blocksData: EmailDocument | null
    handleModeSwitch: (mode: EditorMode) => void
    confirmModeSwitch: () => void
    cancelModeSwitch: () => void
    handleBlocksChange: (doc: EmailDocument, jsxSource: string) => void
}

/**
 * Manages editor mode switching (code / builder / blocks) with confirmation
 * dialogs, block data lifecycle, and selector state.
 */
export function useEditorMode({
    initialMode,
    code,
    setCode,
    editorRef,
    initialBlocksData,
}: UseEditorModeOptions): UseEditorModeResult {
    const [editorMode, setEditorMode] = useState<EditorMode>(initialMode)
    const [pendingMode, setPendingMode] = useState<EditorMode | null>(null)
    const [selectorActive, setSelectorActive] = useState(false)
    const [blockEditorTab, setBlockEditorTab] = useState<BlockEditorTab>("editor")
    const [blocksData, setBlocksData] = useState<EmailDocument | null>(initialBlocksData)

    // Keep a ref to current code so callbacks don't go stale
    const codeRef = useRef(code)
    codeRef.current = code

    const handleModeSwitch = useCallback(
        (mode: EditorMode) => {
            if (mode === editorMode) return

            const leavingBlocks = editorMode === "blocks"
            const enteringBlocks = mode === "blocks"

            if (leavingBlocks || enteringBlocks) {
                setPendingMode(mode)
                return
            }

            // code <-> builder switches don't need confirmation
            if (mode === "code" && editorRef.current) {
                const model = editorRef.current.getModel()
                if (model && model.getValue() !== codeRef.current) {
                    model.setValue(codeRef.current)
                }
            }
            setEditorMode(mode)
            if (mode !== "builder") {
                setSelectorActive(false)
            }
        },
        [editorMode, editorRef],
    )

    const confirmModeSwitch = useCallback(() => {
        if (!pendingMode) return

        const leavingBlocks = editorMode === "blocks"

        if (leavingBlocks) {
            setBlocksData(null)
        }

        if (pendingMode === "code" && editorRef.current) {
            const model = editorRef.current.getModel()
            if (model && model.getValue() !== codeRef.current) {
                model.setValue(codeRef.current)
            }
        }

        setEditorMode(pendingMode)
        if (pendingMode !== "builder") {
            setSelectorActive(false)
        }
        setPendingMode(null)
    }, [pendingMode, editorMode, editorRef])

    const cancelModeSwitch = useCallback(() => {
        setPendingMode(null)
    }, [])

    const handleBlocksChange = useCallback(
        (doc: EmailDocument, jsxSource: string) => {
            setBlocksData(doc)
            setCode(jsxSource)
        },
        [setCode],
    )

    return {
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
    }
}

/**
 * Compute the initial editor mode from template data.
 */
export function getInitialEditorMode(
    templateData: Record<string, unknown> | undefined,
): EditorMode {
    const stored = templateData?.editorMode as EditorMode | undefined
    if (stored === "blocks" && isEnterprise) return "blocks"
    if (stored === "builder" && isEnterprise) return "builder"
    if (stored === "code") return "code"
    return isEnterprise ? "blocks" : "code"
}
