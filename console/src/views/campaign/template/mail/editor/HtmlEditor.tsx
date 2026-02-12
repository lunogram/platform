/* eslint-disable react-hooks/rules-of-hooks */
import { useEffect } from "react";
import CodeEditorEventListener from "./CodeEditorPlugins/CodeEditorEventListener";
import { Puck, type Data } from "@puckeditor/core";
import { viewports } from "./viewport";
import { config, type Components } from "./Handlers/ConfigHandler";
import SaveHandler from "./Handlers/SaveHandler";
import CodeStore from "./CodeEditorPlugins/CodeStore";
import { Preview } from "./Overrides/Preview";
import { CodeEditorPlugin } from "./CodeEditorPlugins/CodeEditorPlugin";
import "./Editor.css";

export function HtmlEditor({
  data,
  html,
}: {
  data: Partial<Data | Data<Components, object>>;
  html?: string;
}) {
  useEffect(() => {
    if (!html) {
      CodeStore.setCode("");
      return;
    };
    
    const handleInitialLoad = () => {
      CodeStore.setCode(html);
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

    setTimeout(() => {
      CodeEditorEventListener.emit("INITIAL_CODE_LOAD");
    }, 200);

    return () =>
      CodeEditorEventListener.removeEventListener(
        "INITIAL_CODE_LOAD",
        handleInitialLoad,
      );
  }, [html]);

  return (
    <div className="w-full h-full hide-puck-outline">
      <Puck
        viewports={viewports}
        config={config}
        data={data}
        ui={{ leftSideBarVisible: false, rightSideBarVisible: false }}
        plugins={[]}
        overrides={{
          outline: () => <></>,
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
          header: () => <></>,
          headerActions: () => <></>,
          drawer: () => <></>,
          puck: ({ children }) => (
            <div className="flex h-screen w-full overflow-hidden bg-background">
              <div className="w-[450px] border-r flex flex-col bg-white">
                <div className="h-16 px-6 flex items-center border-b bg-slate-50/50">
                  <h3 className="text-xs font-bold uppercase tracking-widest text-slate-400">
                    Developer Mode
                  </h3>
                </div>

                <div className="flex-1 overflow-hidden">
                  <CodeEditorPlugin
                    store={CodeStore}
                    eventListener={CodeEditorEventListener}
                  />
                </div>
              </div>

              {/* 2. PUCK AREA */}
              <div className="flex-1 relative puck-container">{children}</div>

              <style>
                {`
                  .puck-container [class*="_PuckLayout-nav"],
                  .puck-container [class*="_Sidebar-resizeHandle"] {
                    display: none !important;
                  }

                  /* Ensure the preview area takes up the whole space */
                  .puck-container [class*="_PuckCanvas"] {
                    width: 100% !important;
                    max-width: 100% !important;
                  }

                  .puck-container [class*="_PuckLayout-inner"] {
                  --puck-side-nav-width: 0px !important;
                  }
                `}
              </style>

              <SaveHandler
                eventListener={CodeEditorEventListener}
                codeStore={CodeStore}
              />
            </div>
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
    </div>
  );
}
