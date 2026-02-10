import { Editor } from "@monaco-editor/react";
import type CodeStore from "./CodeStore";
import type CodeEditorEventListener from "./CodeEditorEventListener";

export function CodeEditorPlugin(props: {
  store?: typeof CodeStore;
  eventListener?: typeof CodeEditorEventListener;
}) {
  const onChange = (value: string) => {
    props.store?.setCode(value);
    props.eventListener?.safeEmit("codeChange", value);
  };

  return <Editor language="html" onChange={(value) => onChange(value ?? "")} />;
}
