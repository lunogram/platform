/* eslint-disable react-hooks/rules-of-hooks */
import { Puck, type Plugin } from "@puckeditor/core";
import { viewports } from "./viewport";
import { useContext, useEffect } from "react";

import "@puckeditor/core/dist/index.css";
import "./Editor.css";
import { TemplateContext } from "@/contexts";
import { Code2Icon } from "lucide-react";

import SaveHandler from "./Handlers/SaveHandler";
import { config } from "./Handlers/ConfigHandler";
import { CodeEditorPlugin } from "./CodeEditorPlugins/CodeEditorPlugin";
import { Preview } from "./Overrides/Preview";
import CodeEditorEventListener from "./CodeEditorPlugins/CodeEditorEventListener";
import CodeStore from "./CodeEditorPlugins/CodeStore";

const plugin: Plugin = {
  name: "raw-html",
  label: "Raw html",
  icon: <Code2Icon />,
  render: () => {
    return (
      <CodeEditorPlugin
        store={CodeStore}
        eventListener={CodeEditorEventListener}
      />
    );
  },
};

export default function Editor() {
  const [template] = useContext(TemplateContext);
  const data = template.data.editor ?? { content: [], root: {} };

  if (template.data.html) {
    CodeStore.setCode(template.data.html);
    CodeEditorEventListener.emit("codeChange", template.data.html);
  }

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
              <SaveHandler
                eventListener={CodeEditorEventListener}
                codeStore={CodeStore}
              />
              {children}
            </>
          ),
          preview: ({ children }) => {
            return (
              <Preview
                eventListener={CodeEditorEventListener}
                codeStore={CodeStore}
              >
                {children}
              </Preview>
            );
          },
        }}
      />
    </div>
  );
}
