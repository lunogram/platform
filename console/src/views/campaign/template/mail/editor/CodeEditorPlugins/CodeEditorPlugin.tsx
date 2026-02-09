import { Editor } from "@monaco-editor/react";
import { useEditorRef } from "./editorRef";
import { debouncedPublish } from "./debouncedPulish";

export function CodeEditorPlugin() {
  const editorRef = useEditorRef();

  function onChange(value: string) {
    editorRef.current = value;
    debouncedPublish(value);
  }

  return <Editor onChange={(value) => onChange(value ?? "")} />;
}
