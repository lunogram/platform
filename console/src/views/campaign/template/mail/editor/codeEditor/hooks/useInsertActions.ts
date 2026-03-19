import { useCallback, useState } from "react"
import type { Image } from "@/types"
import type { EditorTab } from "../types"
import type { PlainTextEditorRef } from "../PlainTextEditor"

interface UseInsertActionsOptions {
    activeTab: EditorTab
    insertAtCursorCode: (text: string) => void
    plainTextEditorRef: React.RefObject<PlainTextEditorRef | null>
}

export interface UseInsertActionsResult {
    imageModalOpen: boolean
    setImageModalOpen: React.Dispatch<React.SetStateAction<boolean>>
    insertAtCursor: (text: string) => void
    insertVariable: (path: string) => void
    insertImage: (image: Image) => void
}

/**
 * Routes insert operations (text, variables, images) to the correct
 * editor (Monaco code editor or plain text editor) based on the active tab.
 */
export function useInsertActions({
    activeTab,
    insertAtCursorCode,
    plainTextEditorRef,
}: UseInsertActionsOptions): UseInsertActionsResult {
    const [imageModalOpen, setImageModalOpen] = useState(false)

    const insertAtCursor = useCallback(
        (text: string) => {
            if (activeTab === "code") {
                insertAtCursorCode(text)
            } else {
                plainTextEditorRef.current?.insertAtCursor(text)
            }
        },
        [activeTab, insertAtCursorCode, plainTextEditorRef],
    )

    const insertVariable = useCallback(
        (path: string) => {
            if (activeTab === "plaintext") {
                plainTextEditorRef.current?.insertAtCursor(`{{ ${path} }}`)
                return
            }

            // Code editor: insert as JSX expression using props access
            const cleanPath = path.split(" ")[0] // handle "now | date"
            const expression = `{props.${cleanPath}}`
            insertAtCursorCode(expression)
        },
        [activeTab, insertAtCursorCode, plainTextEditorRef],
    )

    const insertImage = useCallback(
        (image: Image) => {
            const url = image.url
            const alt = image.name || image.filename
            if (activeTab === "code") {
                insertAtCursor(`<Img src="${url}" alt="${alt}" width="600" />`)
            } else {
                insertAtCursor(url)
            }
            setImageModalOpen(false)
        },
        [activeTab, insertAtCursor],
    )

    return {
        imageModalOpen,
        setImageModalOpen,
        insertAtCursor,
        insertVariable,
        insertImage,
    }
}
