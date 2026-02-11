/* eslint-disable react-hooks/rules-of-hooks */
import { useEffect } from "react";
import CodeEditorEventListener from "./CodeEditorPlugins/CodeEditorEventListener";
import { Puck, type Data, type Plugin } from "@puckeditor/core";
import { viewports } from "./viewport";
import { config, type Components } from "./Handlers/ConfigHandler";
import SaveHandler from "./Handlers/SaveHandler";
import CodeStore from "./CodeEditorPlugins/CodeStore";
import { Preview } from "./Overrides/Preview";
import { Code2Icon } from "lucide-react";
import { CodeEditorPlugin } from "./CodeEditorPlugins/CodeEditorPlugin";

export function HtmlEditor({
  data,
  html,
}: {
  data: Partial<Data | Data<Components, object>>;
  html?: string;
}) {
  // Transfer your specific initial load logic here
  useEffect(() => {
    if (!html) return;
    const handleInitialLoad = () => {
      CodeEditorEventListener.emit("CODE_CHANGE");
      CodeEditorEventListener.removeEventListener(
        "INITIAL_CODE_LOAD",
        handleInitialLoad,
      );
    };
    CodeEditorEventListener.addEventListener(
      "INITIAL_CODE_LOAD",
      handleInitialLoad,
    );

    // Trigger the load event for the plugin to pick up
    setTimeout(() => {
      CodeEditorEventListener.emit("INITIAL_CODE_LOAD");
    }, 200);

    return () =>
      CodeEditorEventListener.removeEventListener(
        "INITIAL_CODE_LOAD",
        handleInitialLoad,
      );
  }, [html]);

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

  return (
    <Puck
      viewports={viewports}
      config={config}
      data={data}
      plugins={[plugin]} // 'plugin' is your raw-html plugin
      ui={{ leftSideBarVisible: false }} // Hides the tabs/outline
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
        headerActions: () => <></>,
        puck: ({ children }) => (
          <>
            <SaveHandler
              eventListener={CodeEditorEventListener}
              codeStore={CodeStore}
            />
            {children}
          </>
        ),
        preview: ({ children }) => (
          <Preview
            eventListener={CodeEditorEventListener}
            codeStore={CodeStore}
          >
            {children}
          </Preview>
        ),
      }}
    />
  );
}
