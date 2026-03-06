import { useEffect, useRef, useState } from "react"
import type CodeEditorEventListener from "../CodeEditorPlugins/CodeEditorEventListener"
import type CodeStore from "../CodeEditorPlugins/CodeStore"

export const Preview = (props: {
    children: React.ReactNode
    eventListener: typeof CodeEditorEventListener
    codeStore: typeof CodeStore
}) => {
    const [rawHtml, setRawHtml] = useState(props.codeStore.current)
    const iframeRef = useRef<HTMLIFrameElement>(null)

    useEffect(() => {
        const handler = () => {
            setRawHtml(props.codeStore.current)
        }

        props.eventListener.addEventListener("CODE_CHANGE", handler)

        return () => {
            props.eventListener.removeEventListener("CODE_CHANGE", handler)
        }
    }, [props.eventListener, props.codeStore])

    useEffect(() => {
        if (iframeRef.current) {
            const doc = iframeRef.current.contentDocument
            if (doc) {
                doc.open()
                doc.write(rawHtml)
                doc.close()
            }
        }
    }, [rawHtml])

    if (!rawHtml || rawHtml.trim() === "") {
        return <>{props.children}</>
    }

    return (
        <>
            <iframe
                ref={iframeRef}
                className="w-full h-full border-0"
                sandbox="allow-scripts allow-same-origin"
                title="Email Preview"
            />
            <div id="temp" className="hidden">
                {props.children}
            </div>
        </>
    )
}
