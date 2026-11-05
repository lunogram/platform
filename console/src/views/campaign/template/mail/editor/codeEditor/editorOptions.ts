import type { editor } from "monaco-editor"

/** Monaco options for the template source editor. */
export const SOURCE_EDITOR_OPTIONS: editor.IStandaloneEditorConstructionOptions = {
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
}
