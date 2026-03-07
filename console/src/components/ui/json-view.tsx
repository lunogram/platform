import { useState, useCallback, useRef, useEffect } from "react"
import { ChevronRight, ChevronDown, Copy, Check } from "lucide-react"
import { cn } from "@/utils"

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }

interface JsonViewProps {
    data: Record<string, unknown> | unknown[] | null | undefined
    className?: string
    defaultExpanded?: boolean
    /** Custom title displayed in the header. */
    title?: string
}

interface JsonNodeProps {
    keyName?: string
    value: JsonValue
    depth: number
    defaultExpanded: boolean
    isLast: boolean
}

function JsonNode({ keyName, value, depth, defaultExpanded, isLast }: JsonNodeProps) {
    const [isExpanded, setIsExpanded] = useState(defaultExpanded || depth < 2)

    const isObject = value !== null && typeof value === "object"
    const isArray = Array.isArray(value)
    const isEmpty = isObject && Object.keys(value).length === 0

    const renderValue = () => {
        if (value === null) {
            return <span className="text-muted-foreground italic">null</span>
        }
        if (typeof value === "boolean") {
            return <span className="text-amber-600 dark:text-amber-400">{value.toString()}</span>
        }
        if (typeof value === "number") {
            return <span className="text-blue-600 dark:text-blue-400">{value}</span>
        }
        if (typeof value === "string") {
            if (value.match(/^https?:\/\//)) {
                return (
                    <a
                        href={value}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-emerald-600 dark:text-emerald-400 hover:underline"
                    >
                        &quot;{value}&quot;
                    </a>
                )
            }
            return (
                <span className="text-emerald-600 dark:text-emerald-400">&quot;{value}&quot;</span>
            )
        }
        return null
    }

    if (!isObject) {
        return (
            <div className="flex items-start leading-6">
                {keyName !== undefined && (
                    <>
                        <span className="text-foreground/80">{keyName}</span>
                        <span className="text-muted-foreground mx-1">:</span>
                    </>
                )}
                {renderValue()}
                {!isLast && <span className="text-muted-foreground">,</span>}
            </div>
        )
    }

    const entries = isArray
        ? (value as JsonValue[]).map((v, i) => [i.toString(), v] as const)
        : Object.entries(value as Record<string, JsonValue>)

    const bracketOpen = isArray ? "[" : "{"
    const bracketClose = isArray ? "]" : "}"

    if (isEmpty) {
        return (
            <div className="flex items-start leading-6">
                {keyName !== undefined && (
                    <>
                        <span className="text-foreground/80">{keyName}</span>
                        <span className="text-muted-foreground mx-1">:</span>
                    </>
                )}
                <span className="text-muted-foreground">
                    {bracketOpen}
                    {bracketClose}
                </span>
                {!isLast && <span className="text-muted-foreground">,</span>}
            </div>
        )
    }

    return (
        <div>
            <div
                className="flex items-start leading-6 cursor-pointer group/node -mx-1 px-1 rounded hover:bg-muted/60 transition-colors"
                onClick={() => setIsExpanded(!isExpanded)}
            >
                <span className="text-muted-foreground mr-0.5 shrink-0 w-4 flex items-center justify-center">
                    {isExpanded ? (
                        <ChevronDown className="h-3.5 w-3.5" />
                    ) : (
                        <ChevronRight className="h-3.5 w-3.5" />
                    )}
                </span>
                {keyName !== undefined && (
                    <>
                        <span className="text-foreground/80">{keyName}</span>
                        <span className="text-muted-foreground mx-1">:</span>
                    </>
                )}
                <span className="text-muted-foreground">{bracketOpen}</span>
                {!isExpanded && (
                    <>
                        <span className="text-muted-foreground/60 mx-1 text-xs">
                            {isArray ? `${entries.length} items` : `${entries.length} keys`}
                        </span>
                        <span className="text-muted-foreground">{bracketClose}</span>
                        {!isLast && <span className="text-muted-foreground">,</span>}
                    </>
                )}
            </div>
            {isExpanded && (
                <>
                    <div className="ml-4 pl-3 border-l border-border/50">
                        {entries.map(([key, val], index) => (
                            <JsonNode
                                key={key}
                                keyName={isArray ? undefined : key}
                                value={val as JsonValue}
                                depth={depth + 1}
                                defaultExpanded={defaultExpanded}
                                isLast={index === entries.length - 1}
                            />
                        ))}
                    </div>
                    <div className="flex leading-6">
                        <span className="text-muted-foreground ml-4">{bracketClose}</span>
                        {!isLast && <span className="text-muted-foreground">,</span>}
                    </div>
                </>
            )}
        </div>
    )
}

export function JsonView({ data, className, defaultExpanded = true, title }: JsonViewProps) {
    const [copied, setCopied] = useState(false)
    const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    // Clean up copy timer on unmount
    useEffect(() => {
        return () => {
            if (copyTimerRef.current) clearTimeout(copyTimerRef.current)
        }
    }, [])

    const handleCopy = useCallback(async () => {
        await navigator.clipboard.writeText(JSON.stringify(data, null, 2))
        setCopied(true)
        if (copyTimerRef.current) clearTimeout(copyTimerRef.current)
        copyTimerRef.current = setTimeout(() => setCopied(false), 2000)
    }, [data])

    if (data === null || data === undefined) {
        return (
            <div
                className={cn(
                    "rounded-md border bg-muted/30 p-4 text-sm text-muted-foreground italic",
                    className,
                )}
            >
                No data
            </div>
        )
    }

    return (
        <div className={cn("relative rounded-md border", className)}>
            {/* Toolbar */}
            <div className="absolute top-2 right-2 z-10 flex items-center gap-1">
                <button
                    type="button"
                    onClick={handleCopy}
                    className="p-1.5 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
                    title="Copy to clipboard"
                >
                    {copied ? (
                        <Check className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                        <Copy className="h-3.5 w-3.5" />
                    )}
                </button>
            </div>

            {/* Header */}
            {title && (
                <div className="px-3 py-2 border-b bg-muted/30">
                    <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        {title}
                    </span>
                </div>
            )}

            {/* Tree */}
            <div className="p-3 font-mono text-[13px] overflow-auto max-h-100">
                <JsonNode
                    value={data as JsonValue}
                    depth={0}
                    defaultExpanded={defaultExpanded}
                    isLast={true}
                />
            </div>
        </div>
    )
}

// Compact inline view for smaller data displays
export function JsonInline({
    data,
    className,
    maxLength = 100,
}: {
    data: unknown
    className?: string
    maxLength?: number
}) {
    const jsonString = JSON.stringify(data)
    const truncated = jsonString.length > maxLength
    const displayString = truncated ? jsonString.substring(0, maxLength) + "..." : jsonString

    return (
        <code
            className={cn(
                "text-xs bg-muted px-1.5 py-0.5 rounded font-mono text-muted-foreground",
                className,
            )}
        >
            {displayString}
        </code>
    )
}
