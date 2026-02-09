import { Editor } from "@monaco-editor/react";
import { useContext, useEffect, useState } from "react";
import { TemplateContext } from "@/contexts";
import { editorEvents } from "../editorEvents";

export const RawHtmlPlugin = () => {
  const [template] = useContext(TemplateContext);
  const initialHtml = template.data.rawHtml ?? "";
  const [html, setHtml] = useState(initialHtml);

  useEffect(() => {
    editorEvents.setHtml(initialHtml);
  }, [initialHtml]);

  const handleChange = (value: string | undefined) => {
    const newHtml = value || "";
    setHtml(newHtml);
    editorEvents.setHtml(newHtml);
  };

  return (
    <Editor
      value={html}
      onChange={handleChange}
      defaultLanguage="html"
      theme="vs-dark"
    />
  );
};
