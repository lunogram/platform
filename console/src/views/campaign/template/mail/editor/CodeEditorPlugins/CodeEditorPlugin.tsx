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
    <Editor
      value={props.store.current}
      language="html"      
      onChange={(value) => onChange(value ?? "")}
    />
  );
}
