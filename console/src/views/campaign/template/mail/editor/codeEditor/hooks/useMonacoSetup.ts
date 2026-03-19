import { useCallback, useEffect, useMemo, useRef } from "react"
import type { OnMount } from "@monaco-editor/react"
import type { editor } from "monaco-editor"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"
import { configureMonaco, updatePropsTypeDeclarations } from "../monacoSetup"
import { generatePropsTypeDeclarations } from "../../variableScope"

export interface UseMonacoSetupResult {
    editorRef: React.RefObject<editor.IStandaloneCodeEditor | null>
    monacoRef: React.RefObject<Parameters<OnMount>[1] | null>
    handleEditorMount: OnMount
    handleCodeChange: (value: string | undefined) => void
    insertAtCursorCode: (text: string) => void
}

/**
 * Manages Monaco editor lifecycle: refs, mount configuration, type
 * declarations for IntelliSense, and cursor insertion helpers.
 */
export function useMonacoSetup(
    variableGroups: VariableGroup[],
    setCode: (code: string) => void,
): UseMonacoSetupResult {
    const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null)
    const monacoRef = useRef<Parameters<OnMount>[1] | null>(null)

    // Generate TypeScript type declarations for IntelliSense
    const propsTypeDeclarations = useMemo(
        () => generatePropsTypeDeclarations(variableGroups),
        [variableGroups],
    )

    // Update Monaco when variable groups change
    useEffect(() => {
        const monaco = monacoRef.current
        if (!monaco) return
        updatePropsTypeDeclarations(monaco, propsTypeDeclarations)
    }, [propsTypeDeclarations])

    const handleEditorMount: OnMount = useCallback(
        (editor, monaco) => {
            editorRef.current = editor
            monacoRef.current = monaco
            configureMonaco(editor, monaco, propsTypeDeclarations)
        },
        [propsTypeDeclarations],
    )

    const handleCodeChange = useCallback(
        (value: string | undefined) => {
            setCode(value ?? "")
        },
        [setCode],
    )

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

    return {
        editorRef,
        monacoRef,
        handleEditorMount,
        handleCodeChange,
        insertAtCursorCode,
    }
}
