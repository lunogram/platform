/* eslint-disable react-hooks/rules-of-hooks */
import { useEffect } from "react"
import { config, type Components } from "../handlers/ConfigHandler"
import { viewports } from "../viewport"
import { Puck, type Data } from "@puckeditor/core"
import CodeEditorEventListener from "../CodeEditorPlugins/CodeEditorEventListener"
import CodeStore from "../CodeEditorPlugins/CodeStore"
import { Preview } from "../Overrides/Preview"
import BlockSaveHandler from "./BlockSaveHandler"

import "./Editor.css"

export function BlockEditor({ data }: { data: Partial<Data | Data<Components, object>> }) {
    return (
        <Puck
            viewports={viewports}
            config={config}
            data={data}
            overrides={{
                iframe: ({ children, document }) => {
                    useEffect(() => {
                        if (document) {
                            const script = document.createElement("script")
                            script.type = "module"
                            script.src = "https://cdn.skypack.dev/twind/shim"
                            document.head.appendChild(script)
                        }
                    }, [document])
                    return <>{children}</>
                },
                headerActions: () => <></>,
                preview: ({ children }) => (
                    <Preview eventListener={CodeEditorEventListener} codeStore={CodeStore}>
                        {children}
                    </Preview>
                ),
                puck: ({ children }) => (
                    <>
                        <BlockSaveHandler />
                        {children}
                    </>
                ),
            }}
        />
    )
}
