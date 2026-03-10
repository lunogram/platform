import { useEffect, useRef, useState } from "react"
import type CodeEditorEventListener from "../codeEditorPlugins/CodeEditorEventListener"
import type CodeStore from "../codeEditorPlugins/CodeStore"

export const Preview = (props: {
    children: React.ReactNode
    eventListener: typeof CodeEditorEventListener
    codeStore: typeof CodeStore
}) => {
    const [html, sethtml] = useState(props.codeStore.current)
    const iframeRef = useRef<HTMLIFrameElement>(null)

    useEffect(() => {
        const handler = () => {
            sethtml(props.codeStore.current)
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
                doc.write(html)
                doc.close()
            }
        }
    }, [html])

    if (!html || html.trim() === "") {
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
