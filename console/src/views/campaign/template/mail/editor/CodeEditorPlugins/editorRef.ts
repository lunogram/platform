import { useRef } from "react";

export function useEditorRef() {
  return useRef<string>("");
}
