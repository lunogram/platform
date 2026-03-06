import * as React from "react"
import { useState, useCallback } from "react"
import { ChevronRight, ChevronDown, Copy, Check, Pencil } from "lucide-react"
import { cn } from "@/utils"
import { Button } from "./button"
import { Textarea } from "./textarea"

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }

interface JsonViewProps {
    data: Record<string, unknown> | unknown[] | null | undefined
    editable?: boolean
    onChange?: (data: Record<string, unknown>) => void
    className?: string
    defaultExpanded?: boolean
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
            // Check if it's a URL
            if (value.match(/^https?:\/\//)) {
                return (
                    <a
                        href={value}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-emerald-600 dark:text-emerald-400 hover:underline"
                    >
                        "{value}"
                    </a>
                )
            }
            return <span className="text-emerald-600 dark:text-emerald-400">"{value}"</span>
        }
        return null
    }

    if (!isObject) {
        return (
            <div className="flex items-start py-0.5">
                {keyName !== undefined && (
                    <>
                        <span className="text-foreground font-medium">{keyName}</span>
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
            <div className="flex items-start py-0.5">
                {keyName !== undefined && (
                    <>
                        <span className="text-foreground font-medium">{keyName}</span>
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
        <div className="py-0.5">
            <div
                className="flex items-start cursor-pointer hover:bg-muted/50 -mx-1 px-1 rounded"
                onClick={() => setIsExpanded(!isExpanded)}
            >
                <span className="text-muted-foreground mr-1 flex-shrink-0 w-4">
                    {isExpanded ? (
                        <ChevronDown className="h-4 w-4" />
                    ) : (
                        <ChevronRight className="h-4 w-4" />
                    )}
                </span>
                {keyName !== undefined && (
                    <>
                        <span className="text-foreground font-medium">{keyName}</span>
                        <span className="text-muted-foreground mx-1">:</span>
                    </>
                )}
                <span className="text-muted-foreground">{bracketOpen}</span>
                {!isExpanded && (
                    <>
                        <span className="text-muted-foreground mx-1">
                            {isArray ? `${entries.length} items` : `${entries.length} keys`}
                        </span>
                        <span className="text-muted-foreground">{bracketClose}</span>
                        {!isLast && <span className="text-muted-foreground">,</span>}
                    </>
                )}
            </div>
            {isExpanded && (
                <>
                    <div className="ml-5 border-l border-border pl-3">
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
                    <div className="flex">
                        <span className="text-muted-foreground ml-5">{bracketClose}</span>
                        {!isLast && <span className="text-muted-foreground">,</span>}
                    </div>
                </>
            )}
        </div>
    )
}

export function JsonView({
    data,
    editable = false,
    onChange,
    className,
    defaultExpanded = true,
}: JsonViewProps) {
    const [copied, setCopied] = useState(false)
    const [isEditing, setIsEditing] = useState(false)
    const [editValue, setEditValue] = useState("")
    const [parseError, setParseError] = useState<string | null>(null)

    const handleCopy = useCallback(async () => {
        await navigator.clipboard.writeText(JSON.stringify(data, null, 2))
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }, [data])

    const handleEdit = useCallback(() => {
        setEditValue(JSON.stringify(data, null, 2))
        setParseError(null)
        setIsEditing(true)
    }, [data])

    const handleSave = useCallback(() => {
        try {
            const parsed = JSON.parse(editValue)
            onChange?.(parsed)
            setIsEditing(false)
            setParseError(null)
        } catch {
            setParseError("Invalid JSON")
        }
    }, [editValue, onChange])

    const handleCancel = useCallback(() => {
        setIsEditing(false)
        setParseError(null)
    }, [])

    if (data === null || data === undefined) {
        return (
            <div
                className={cn(
                    "rounded-lg border bg-muted/30 p-4 text-sm text-muted-foreground italic",
                    className,
                )}
            >
                No data
            </div>
        )
    }

    if (isEditing) {
        return (
            <div className={cn("rounded-lg border bg-card", className)}>
                <div className="border-b bg-muted/30 px-3 py-2 flex items-center justify-between">
                    <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        Edit JSON
                    </span>
                    <div className="flex items-center gap-2">
                        <Button variant="ghost" size="sm" onClick={handleCancel}>
                            Cancel
                        </Button>
                        <Button size="sm" onClick={handleSave}>
                            Save
                        </Button>
                    </div>
                </div>
                <div className="p-3">
                    <Textarea
                        value={editValue}
                        onChange={(e) => {
                            setEditValue(e.target.value)
                            setParseError(null)
                        }}
                        className="font-mono text-sm min-h-[200px] resize-y"
                        placeholder="Enter JSON..."
                    />
                    {parseError && <p className="text-sm text-destructive mt-2">{parseError}</p>}
                </div>
            </div>
        )
    }

    return (
        <div className={cn("rounded-lg border bg-card", className)}>
            <div className="border-b bg-muted/30 px-3 py-2 flex items-center justify-between">
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                    JSON
                </span>
                <div className="flex items-center gap-1">
                    {editable && onChange && (
                        <Button variant="ghost" size="sm" onClick={handleEdit} className="h-7 px-2">
                            <Pencil className="h-3.5 w-3.5" />
                        </Button>
                    )}
                    <Button variant="ghost" size="sm" onClick={handleCopy} className="h-7 px-2">
                        {copied ? (
                            <Check className="h-3.5 w-3.5 text-green-500" />
                        ) : (
                            <Copy className="h-3.5 w-3.5" />
                        )}
                    </Button>
                </div>
            </div>
            <div className="p-3 font-mono text-sm overflow-auto max-h-[400px]">
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
