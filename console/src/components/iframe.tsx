import { useCallback, useEffect, useRef } from "react"

interface IframeProps {
    content: string
    fullHeight?: boolean
    allowScroll?: boolean
    width?: string
}

export default function Iframe({
    content,
    fullHeight = false,
    allowScroll = true,
    width,
}: IframeProps) {
    const ref = useRef<HTMLIFrameElement>(null)

    const setBody = useCallback(() => {
        const frame = ref.current
        if (frame) {
            if (frame.contentDocument?.body) {
                frame.contentDocument.body.innerHTML = content
                frame.contentDocument.body.style.overflow = allowScroll ? "" : "hidden"
                frame.contentDocument.documentElement.style.overflow = allowScroll ? "" : "hidden"
            }
            if (fullHeight) {
                frame.style.minHeight = `${frame.contentWindow?.document.documentElement.scrollHeight}px`
            }
        }
    }, [allowScroll, content, fullHeight])

    useEffect(() => setBody(), [content, setBody])

    return (
        <iframe
            src="about:blank"
            frameBorder="0"
            scrolling={allowScroll ? "yes" : "no"}
            sandbox="allow-scripts allow-same-origin"
            ref={ref}
            style={{ ...(width ? { width } : {}), overflow: allowScroll ? "auto" : "hidden" }}
            onLoad={() => setBody()}
        />
    )
}
