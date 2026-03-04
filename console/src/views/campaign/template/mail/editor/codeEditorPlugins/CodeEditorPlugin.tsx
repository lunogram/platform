import { Editor, type OnMount } from "@monaco-editor/react";
import type { editor } from "monaco-editor";
import type CodeStore from "./CodeStore";
import type CodeEditorEventListener from "./CodeEditorEventListener";

export function CodeEditorPlugin(props: {
  store: typeof CodeStore;
  eventListener: typeof CodeEditorEventListener;
  editorRef?: React.MutableRefObject<editor.IStandaloneCodeEditor | null>;
}) {
  const onChange = (value: string) => {
    props.store.setCode(value);
    props.eventListener.safeEmit("CODE_CHANGE");
  };

  const handleEditorMount: OnMount = (editor) => {
    if (props.editorRef) {
      props.editorRef.current = editor;
    }
  };

  return (
    <div className="h-full w-full">
      <Editor
        value={props.store.current}
        language="html"
        // theme="vs-dark"
        onChange={(value) => onChange(value ?? "")}
        onMount={handleEditorMount}
        options={{
          automaticLayout: true,
          minimap: { enabled: false },
          fontSize: 14,
          scrollBeyondLastLine: false,
        }}
      />
    </div>
  );
}