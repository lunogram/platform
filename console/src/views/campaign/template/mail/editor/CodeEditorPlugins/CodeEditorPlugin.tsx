import { Editor } from "@monaco-editor/react";
import type CodeStore from "./CodeStore";
import type CodeEditorEventListener from "./CodeEditorEventListener";

export function CodeEditorPlugin(props: {
  store: typeof CodeStore;
  eventListener: typeof CodeEditorEventListener;
}) {
  const onChange = (value: string) => {
    props.store.setCode(value);
    props.eventListener.safeEmit("CODE_CHANGE");
  };

  return (
    <div className="h-full w-full">
      <Editor
        value={props.store.current}
        language="html"
        // theme="vs-dark"
        onChange={(value) => onChange(value ?? "")}
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