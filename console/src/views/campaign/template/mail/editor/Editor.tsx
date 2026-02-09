/* eslint-disable react-hooks/rules-of-hooks */
import { Puck, type Plugin } from "@puckeditor/core";
import { viewports } from "./viewport";
import { useContext, useEffect } from "react";

import "@puckeditor/core/dist/index.css";
import "./Editor.css";
import { TemplateContext } from "@/contexts";
import { Code2Icon } from "lucide-react";
import { editorEvents } from "./editorEvents";

import SaveHandler from "./Handlers/SaveHandler";
import { config } from "./Handlers/ConfigHandler";
import { CodeEditorPlugin } from "./CodeEditorPlugins/CodeEditorPlugin";
import { StablePreview } from "./CodeEditorPlugins/StablePreview";

const plugin: Plugin = {
  name: "raw-html",
  label: "Raw html",
  icon: <Code2Icon />,
  render: () => {
    return <CodeEditorPlugin />;
  },
};

export default function Editor() {
  const [template] = useContext(TemplateContext);
  const data = template.data.editor ?? { content: [], root: {} };

  useEffect(() => {
    return () => editorEvents.reset();
  }, []);

  return (
    <div className="w-full h-full">
      <Puck
        viewports={viewports}
        config={config}
        data={data}
        plugins={[plugin]}
        overrides={{
          iframe: ({ children, document }) => {
            useEffect(() => {
              if (document) {
                const script = document.createElement("script");
                script.type = "module";
                script.src = "https://cdn.skypack.dev/twind/shim";
                document.head.appendChild(script);
              }
            }, [document]);

            return <>{children}</>;
          },
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          headerActions: ({ children }) => <></>,
          puck: ({ children }) => (
            <>
              <SaveHandler />
              {children}
            </>
          ),
          preview: StablePreview,
        }}
      />
    </div>
  );
}
