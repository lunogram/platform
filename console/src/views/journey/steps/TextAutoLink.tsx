import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import remarkBreaks from "remark-breaks"
import rehypeRaw from "rehype-raw"
import type { Components } from "react-markdown"

const styles = {
    h1: { fontSize: "2em", fontWeight: "bold" as const, margin: "0.67em 0" },
    h2: { fontSize: "1.5em", fontWeight: "bold" as const, margin: "0.75em 0" },
    h3: { fontSize: "1.17em", fontWeight: "bold" as const, margin: "0.83em 0" },
    h4: { fontSize: "1em", fontWeight: "bold" as const, margin: "1.12em 0" },
    h5: { fontSize: "0.83em", fontWeight: "bold" as const, margin: "1.5em 0" },
    h6: { fontSize: "0.75em", fontWeight: "bold" as const, margin: "1.67em 0" },
    code: {
        fontFamily: "monospace",
        backgroundColor: "rgba(0,0,0,0.08)",
        borderRadius: "4px",
        padding: "1px 5px",
        fontSize: "0.875em",
    },
    blockquote: {
        borderLeft: "3px solid #ccc",
        paddingLeft: "0.75em",
        margin: "0.25em 0",
        color: "#666",
    },
    ul: { listStyleType: "disc", paddingLeft: "1.5em", margin: "0.25em 0" },
    ol: { listStyleType: "decimal", paddingLeft: "1.5em", margin: "0.25em 0" },
    a: { color: "#3b82f6", textDecoration: "underline" },
    del: { textDecoration: "line-through" },
    p: { fontSize: "0.875em", margin: "0.75em 0" },
}

const components: Components = {
    h1: ({ children }) => <h1 style={styles.h1}>{children}</h1>,
    h2: ({ children }) => <h2 style={styles.h2}>{children}</h2>,
    h3: ({ children }) => <h3 style={styles.h3}>{children}</h3>,
    h4: ({ children }) => <h4 style={styles.h4}>{children}</h4>,
    h5: ({ children }) => <h5 style={styles.h5}>{children}</h5>,
    h6: ({ children }) => <h6 style={styles.h6}>{children}</h6>,
    code: ({ className, children }) => {
        const isInline = !className
        if (isInline) {
            return <code style={styles.code}>{children}</code>
        }
        return <code className={className}>{children}</code>
    },
    blockquote: ({ children }) => <blockquote style={styles.blockquote}>{children}</blockquote>,
    ul: ({ children }) => <ul style={styles.ul}>{children}</ul>,
    ol: ({ children }) => <ol style={styles.ol}>{children}</ol>,
    a: ({ href, children }) => {
        if (!href) return <span>{children}</span>
        return (
            <a
                style={styles.a}
                href={href.startsWith("http") ? href : `https://${href}`}
                target="_blank"
                rel="noreferrer"
            >
                {children}
            </a>
        )
    },
    del: ({ children }) => <del style={styles.del}>{children}</del>,
    hr: () => <hr />,
    p: ({ children }) => <p style={styles.p}>{children}</p>,
    table: ({ children }) => (
        <table style={{ borderCollapse: "collapse", width: "100%" }}>{children}</table>
    ),
    th: ({ children }) => (
        <th style={{ border: "1px solid #ccc", padding: "6px", textAlign: "left" }}>{children}</th>
    ),
    td: ({ children }) => <td style={{ border: "1px solid #ccc", padding: "6px" }}>{children}</td>,
    tr: ({ children }) => <tr>{children}</tr>,
}

const PLAIN_DOMAIN_REGEX =
    /(?<![.\w@/])(\b[a-zA-Z0-9][a-zA-Z0-9-]*\.[a-zA-Z]{2,}(?:\/[^\s]*)?)(?![-@])/g

const remarkNoSetextHeadings = () => {}
remarkNoSetextHeadings.data = {
    micromarkExtensions: [{ disable: { null: ["setextUnderline"] } }],
}

const preprocessText = (text: string): string => {
    const withLinks = text.replace(PLAIN_DOMAIN_REGEX, (match) => `[${match}](https://${match})`)
    const withBreaks = "\n" + withLinks + "\n"

    return withBreaks.replace(/\n\n+/g, (match) => {
        const numNewlines = match.length
        const numBlankLines = Math.ceil(numNewlines / 2) - 1

        if (numBlankLines <= 0) return "\n\n"
        return "\n\n" + "&nbsp;\n\n".repeat(numBlankLines)
    })
}

const TextAutoLink = ({ text }: { text: string }) => (
    <>
        <ReactMarkdown
            remarkPlugins={[remarkGfm, remarkBreaks, remarkNoSetextHeadings]}
            rehypePlugins={[rehypeRaw]}
            components={components}
        >
            {preprocessText(text)}
        </ReactMarkdown>
    </>
)

export default TextAutoLink
