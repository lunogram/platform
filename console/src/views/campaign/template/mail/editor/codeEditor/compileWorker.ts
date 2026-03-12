/**
 * Web Worker that runs the React Email compilation pipeline off the main thread.
 *
 * This isolates the `new Function()` eval in a separate global scope (no DOM,
 * cookies, localStorage, or other main-thread APIs), and prevents complex
 * templates from blocking the UI during the Sucrase + React render steps.
 *
 * Communication uses structured postMessage:
 *   Main → Worker:  { id, source, previewProps }
 *   Worker → Main:  { id, html, plainText } | { id, error }
 */

// Must be first: prevents Prism.js (inside @react-email/components) from
// hijacking the worker's message handler with an incompatible JSON.parse call.
import "./disablePrismWorker"

import React from "react"
import { jsx, jsxs, Fragment } from "react/jsx-runtime"
import { transform } from "sucrase"
import { render } from "@react-email/render"
import * as ReactEmailComponents from "@react-email/components"

import {
    extractCustomFonts,
    extractFontClassNames,
    buildGoogleFontsCss,
    injectFontCss,
    buildFontAwareTailwindConfig,
} from "./fontUtils"
import { cleanStreamingHtml } from "./htmlUtils"

interface CompileRequest {
    id: number
    source: string
    previewProps: Record<string, unknown>
}

interface CompileSuccess {
    id: number
    html: string
    plainText: string
}

interface CompileError {
    id: number
    error: string
}

function createTailwindWrapper(): {
    Wrapper: React.ComponentType<Record<string, unknown>>
    getCapturedConfig: () => Record<string, unknown> | null
} {
    let capturedConfig: Record<string, unknown> | null = null
    const OriginalTailwind = ReactEmailComponents.Tailwind

    function Wrapper(props: Record<string, unknown>) {
        if (props.config && typeof props.config === "object") {
            capturedConfig = props.config as Record<string, unknown>
        }
        return React.createElement(
            OriginalTailwind,
            props as unknown as React.ComponentProps<typeof OriginalTailwind>,
        )
    }

    return {
        Wrapper,
        getCapturedConfig: () => capturedConfig,
    }
}

function createSafeProps(obj: Record<string, unknown>): Record<string, unknown> {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const handler: ProxyHandler<any> = {
        get(target, prop) {
            if (typeof prop === "symbol" || prop === "toJSON") {
                return Reflect.get(target, prop)
            }
            const value = target[prop as string]
            if (value === undefined || value === null) {
                const stub = Object.assign(() => "", { toString: () => "", valueOf: () => "" })
                return new Proxy(stub, handler)
            }
            if (typeof value === "object" && !Array.isArray(value)) {
                return createSafeProps(value as Record<string, unknown>)
            }
            return value
        },
    }
    return new Proxy(obj, handler)
}

const REACT_EMAIL_SCOPE: Record<string, unknown> = {
    _jsx: jsx,
    _jsxs: jsxs,
    _Fragment: Fragment,
    React,
    ...ReactEmailComponents,
}

async function compile(
    source: string,
    previewProps: Record<string, unknown>,
): Promise<{ html: string; plainText: string }> {
    // 1. Transpile JSX/TypeScript
    const transformed = transform(source, {
        transforms: ["jsx", "typescript"],
        jsxRuntime: "automatic",
        jsxImportSource: "react",
        production: true,
    })

    let execCode = transformed.code

    // 2. Detect tailwind.config imports before stripping
    const tailwindConfigBindings: string[] = []
    const tailwindImportRe =
        /^import\s+(\w+)\s+from\s+['"].*tailwind\.config(?:\.ts|\.js|\.mjs)?['"];?\s*$/gm
    let twMatch: RegExpExecArray | null
    while ((twMatch = tailwindImportRe.exec(execCode)) !== null) {
        tailwindConfigBindings.push(twMatch[1])
    }

    // 3. Strip import/export statements
    execCode = execCode.replace(/^import\s+[\s\S]*?from\s+['"].*?['"];?\s*$/gm, "")
    execCode = execCode.replace(/^import\s+['"].*?['"];?\s*$/gm, "")
    execCode = execCode.replace(/export\s+default\s+/, "var __Component__ = ")
    execCode = execCode.replace(/export\s+(?:const|let|var|function|class)\s+/g, "var ")

    // 4. Build scope with Tailwind wrapper
    const { Wrapper: TailwindWrapper, getCapturedConfig } = createTailwindWrapper()

    const fullScope: Record<string, unknown> = {
        ...REACT_EMAIL_SCOPE,
        Tailwind: TailwindWrapper,
    }

    if (tailwindConfigBindings.length > 0) {
        const { config: defaultTailwindConfig } = buildFontAwareTailwindConfig(null, source)
        for (const binding of tailwindConfigBindings) {
            fullScope[binding] = defaultTailwindConfig
        }
    }

    const scopeKeys = Object.keys(fullScope)
    const scopeValues = scopeKeys.map((k) => fullScope[k])

    // 5. Execute in controlled scope (isolated in this worker's global)
    const fn = new Function(
        ...scopeKeys,
        `${execCode}\nreturn typeof __Component__ !== 'undefined' ? __Component__ : null;`,
    )
    const Component = fn(...scopeValues) as React.ComponentType | null

    if (!Component) {
        throw new Error("No default export found. Make sure to export a default function.")
    }

    // 6. Render to HTML
    function SafeComponent(props: Record<string, unknown>) {
        return (Component as (p: Record<string, unknown>) => React.ReactElement)(
            createSafeProps({ ...previewProps, ...props }),
        )
    }
    const element = React.createElement(SafeComponent, previewProps)
    const rawHtml = await render(element)

    // 7. Extract font config
    let detectedFonts: string[] = []
    const capturedConfig = getCapturedConfig()
    if (capturedConfig) {
        detectedFonts = extractCustomFonts(capturedConfig)
    }
    if (detectedFonts.length === 0) {
        detectedFonts = extractFontClassNames(source)
    }

    // 8. Clean streaming HTML artifacts
    let cleanedHtml = cleanStreamingHtml(rawHtml)

    // 9. Inject Google Fonts CSS
    if (detectedFonts.length > 0) {
        const fontCss = buildGoogleFontsCss(detectedFonts)
        cleanedHtml = injectFontCss(cleanedHtml, fontCss)
    }

    // 10. Generate plain text
    const plainText = await render(element, { plainText: true })

    return { html: cleanedHtml, plainText }
}

self.onmessage = async (e: MessageEvent<CompileRequest>) => {
    const { id, source, previewProps } = e.data
    try {
        const result = await compile(source, previewProps)
        const response: CompileSuccess = { id, ...result }
        self.postMessage(response)
    } catch (err) {
        const response: CompileError = {
            id,
            error: err instanceof Error ? err.message : String(err),
        }
        self.postMessage(response)
    }
}
