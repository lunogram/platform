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
                // Collapse before measuring. A document can never report a
                // scrollHeight smaller than the frame rendering it, so measuring
                // in place only ever grows the frame — a short document inherits
                // the height of whatever was shown before it, and the first
                // measurement is pinned to the 300x150 default an iframe gets
                // when no size is set.
                const root = frame.contentDocument?.documentElement
                if (root) {
                    const previous = frame.style.height
                    frame.style.height = "0px"
                    const contentHeight = root.scrollHeight
                    // Keep the previous height rather than collapsing to nothing
                    // if the document is not measurable yet.
                    frame.style.height = contentHeight > 0 ? `${contentHeight}px` : previous
                }
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
