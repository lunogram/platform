import { useCallback, useRef, useState } from "react"
import type { editor } from "monaco-editor"
import { isEnterprise } from "@/config/enterprise"

/**
 * The visual editor's document, kept opaque here — these hooks only pass it
 * around. Its concrete shape is Templatical's `TemplateContent`.
 */
export type EmailDocument = Record<string, unknown>
export type BlockEditorTab = "editor" | "preview-text"

/**
 * Identifier for the visual editing mode, persisted per template in
 * `data.editorMode`. Declared once so renaming the mode is a single edit plus
 * a read-side alias in getInitialEditorMode for already-stored templates.
 */
export const BLOCKS_MODE = "blocks"

/** The top-level editing mode: code editor, AI builder, or block editor */
export type EditorMode = "code" | "builder" | typeof BLOCKS_MODE

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
    handleBlocksChange: (doc: EmailDocument, jsxSource?: string) => void
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

        // The document is deliberately kept when leaving the visual editor.
        // The JSX survives a switch in the other direction (persistence always
        // writes code.source), so keeping both makes the switch reversible in
        // both directions and nothing the user authored is destroyed. Only
        // data.type decides which one the backend compiles.

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
    }, [pendingMode, editorRef])

    const cancelModeSwitch = useCallback(() => {
        setPendingMode(null)
    }, [])

    const handleBlocksChange = useCallback(
        (doc: EmailDocument, jsxSource?: string) => {
            setBlocksData(doc)
            // The enterprise block editor emits a JSX representation of the
            // document. Templatical has none — the backend renders its document
            // to HTML directly — so `code` is left untouched, which also
            // preserves any JSX the template had before switching.
            if (jsxSource !== undefined) {
                setCode(jsxSource)
            }
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
    if (stored === BLOCKS_MODE) return BLOCKS_MODE
    if (stored === "builder" && isEnterprise) return "builder"
    if (stored === "code") return "code"
    // Templates saved before a mode was recorded are React Email. Anything
    // created from now on has its mode chosen at creation time.
    return "code"
}
