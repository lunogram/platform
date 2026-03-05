import type { PropsWithChildren } from "react"
import { createElement } from "react"

type TextProps = PropsWithChildren<{
    html?: string
    variant?: "normal" | "secondary" | "h1" | "h2" | "h3" | "h4"
    element?: keyof HTMLElementTagNameMap
}>

export default function Text({ children, element = "div", html }: TextProps) {
    if (html != null) {
        return createElement(element, {
            dangerouslySetInnerHTML: { __html: html },
        })
    }

    return createElement(element, {}, children)
}
