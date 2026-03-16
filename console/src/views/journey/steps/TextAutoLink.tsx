import React from "react"

// every pattern that makes text look less like a wall of nothing
const BOLD_REGEX = /\*\*(.+?)\*\*/g
const ITALIC_REGEX = /(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g
const CODE_REGEX = /`([^`]+)`/g
const STRIKETHROUGH_REGEX = /~~(.+?)~~/g
const HEADING_REGEX = /^(#{1,6})\s+(.+)/
const BLOCKQUOTE_REGEX = /^>\s+(.+)/
const HR_REGEX = /^(-{3,}|\*{3,}|_{3,})$/
const UNORDERED_LIST_REGEX = /^[-*+]\s+(.+)/
const ORDERED_LIST_REGEX = /^(\d+)\.\s+(.+)/

const HEADING_STYLES: Record<number, React.CSSProperties> = {
    1: { fontSize: "2em", fontWeight: "bold", margin: "0.67em 0" },
    2: { fontSize: "1.5em", fontWeight: "bold", margin: "0.75em 0" },
    3: { fontSize: "1.17em", fontWeight: "bold", margin: "0.83em 0" },
    4: { fontSize: "1em", fontWeight: "bold", margin: "1.12em 0" },
    5: { fontSize: "0.83em", fontWeight: "bold", margin: "1.5em 0" },
    6: { fontSize: "0.75em", fontWeight: "bold", margin: "1.67em 0" },
}

const URL_DELIMITER =
    /((?:https?:\/\/)?(?:(?:[a-z0-9]?(?:[a-z0-9-]{1,61}[a-z0-9])?\.[^.|\s])+[a-z.]*[a-z]+|(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3})(?::\d{1,5})*[a-z0-9.,_/~#&=;%+?\-()]*)/gi

const linkify = (text: string, lineIdx: number): React.ReactNode[] =>
    text
        .split(URL_DELIMITER)
        .map((chunk, chunkIdx) => {
            const match = chunk.match(URL_DELIMITER)
            if (match) {
                const url = match[0]
                return (
                    <a
                        key={`${lineIdx}-url-${chunkIdx}`}
                        target="_blank"
                        style={{ color: "#3b82f6", textDecoration: "underline" }}
                        href={url.startsWith("http") ? url : `https://${url}`}
                        rel="noreferrer"
                    >
                        {url}
                    </a>
                )
            }
            return chunk || null
        })
        .filter(Boolean)

const parseInline = (text: string, lineIdx: number, inlineIdx = 0): React.ReactNode[] => {
    const patterns: [RegExp, (match: RegExpExecArray) => React.ReactNode][] = [
        [
            BOLD_REGEX,
            (m) => (
                <strong key={`${lineIdx}-b-${inlineIdx}`}>
                    {parseInline(m[1], lineIdx, inlineIdx + 1)}
                </strong>
            ),
        ],
        [
            ITALIC_REGEX,
            (m) => (
                <em key={`${lineIdx}-i-${inlineIdx}`}>
                    {parseInline(m[1], lineIdx, inlineIdx + 2)}
                </em>
            ),
        ],
        [
            CODE_REGEX,
            (m) => (
                <code
                    key={`${lineIdx}-c-${inlineIdx}`}
                    style={{
                        fontFamily: "monospace",
                        backgroundColor: "rgba(0,0,0,0.08)",
                        borderRadius: "4px",
                        padding: "1px 5px",
                        fontSize: "0.875em",
                    }}
                >
                    {m[1]}
                </code>
            ),
        ],
        [
            STRIKETHROUGH_REGEX,
            (m) => (
                <del key={`${lineIdx}-s-${inlineIdx}`}>
                    {parseInline(m[1], lineIdx, inlineIdx + 3)}
                </del>
            ),
        ],
    ]

    let earliest: {
        index: number
        match: RegExpExecArray
        render: (m: RegExpExecArray) => React.ReactNode
    } | null = null

    for (const [regex, render] of patterns) {
        regex.lastIndex = 0
        const match = regex.exec(text)
        if (match && (earliest === null || match.index < earliest.index)) {
            earliest = { index: match.index, match, render }
        }
    }

    if (!earliest) return linkify(text, lineIdx)

    const { index, match, render } = earliest
    const nodes: React.ReactNode[] = []

    if (index > 0) nodes.push(...linkify(text.slice(0, index), lineIdx))
    nodes.push(render(match))
    nodes.push(...parseInline(text.slice(index + match[0].length), lineIdx, inlineIdx + 10))

    return nodes
}

const parseLine = (line: string, idx: number): React.ReactNode => {
    if (HR_REGEX.test(line)) return <hr key={idx} />

    const heading = line.match(HEADING_REGEX)
    if (heading) {
        const level = heading[1].length as 1 | 2 | 3 | 4 | 5 | 6
        const Tag = `h${level}` as `h${1 | 2 | 3 | 4 | 5 | 6}`
        return (
            <Tag key={idx} style={HEADING_STYLES[level]}>
                {parseInline(heading[2], idx)}
            </Tag>
        )
    }

    const blockquote = line.match(BLOCKQUOTE_REGEX)
    if (blockquote)
        return (
            <blockquote
                key={idx}
                style={{
                    borderLeft: "3px solid #ccc",
                    paddingLeft: "0.75em",
                    margin: "0.25em 0",
                    color: "#666",
                }}
            >
                {parseInline(blockquote[1], idx)}
            </blockquote>
        )

    const ul = line.match(UNORDERED_LIST_REGEX)
    if (ul)
        return (
            <ul
                key={idx}
                style={{ listStyleType: "disc", paddingLeft: "1.5em", margin: "0.25em 0" }}
            >
                <li>{parseInline(ul[1], idx)}</li>
            </ul>
        )

    const ol = line.match(ORDERED_LIST_REGEX)
    if (ol)
        return (
            <ol
                key={idx}
                start={Number(ol[1])}
                style={{ listStyleType: "decimal", paddingLeft: "1.5em", margin: "0.25em 0" }}
            >
                <li>{parseInline(ol[2], idx)}</li>
            </ol>
        )

    if (line === "") return <br key={idx} />

    return <p key={idx}>{parseInline(line, idx)}</p>
}

const TextAutoLink = ({ text }: { text: string }) => (
    <>{text.split(/\r?\n/).map((line, idx) => parseLine(line, idx))}</>
)

export default TextAutoLink
