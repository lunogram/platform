/* eslint-disable react-hooks/rules-of-hooks */
import { useEffect, useContext } from "react"
import { config, type Components } from "../handlers/ConfigHandler"
import { viewports } from "../viewport"
import { Puck, type Data } from "@puckeditor/core"
import CodeEditorEventListener from "../codeEditorPlugins/CodeEditorEventListener"
import CodeStore from "../codeEditorPlugins/CodeStore"
import { Preview } from "../overrides/Preview"
import BlockSaveHandler from "./BlockSaveHandler"
import { renderBlockToHtml } from "../handlers/renderBlockToHtml"
import { TemplateContext } from "@/mod"

import "./Editor.css"

export function BlockEditor({ data }: { data: Partial<Data | Data<Components, object>> }) {
    const [template] = useContext(TemplateContext)

    const updatePreview = async (puckData: Data) => {
        const html = await renderBlockToHtml(puckData, template.locale)
        CodeStore.setCode(html)
        CodeEditorEventListener.emit("CODE_CHANGE")
    }

    useEffect(() => {
        if (data && data.content) {
            updatePreview(data as Data)
        }
    }, [])

    return (
        <Puck
            viewports={viewports}
            config={config}
            data={data}
            onChange={(newData) => {
                updatePreview(newData as Data)
            }}
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
