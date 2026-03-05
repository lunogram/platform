import { useState, useEffect, useCallback, useRef } from "react"
import { useTranslation } from "react-i18next"
import { Braces, Plus, Trash2, ChevronRight, List, Code } from "lucide-react"

import { Button } from "@/components/ui/button"
import { JsonEditor } from "@/components/ui/json-editor"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { cn } from "@/utils"

type ValueType = "string" | "number" | "boolean" | "object" | "array"

interface TreeNode {
    id: string
    key: string
    type: ValueType
    value: string
    children: TreeNode[]
    expanded: boolean
}

function generateId() {
    return Math.random().toString(36).substring(2, 9)
}

function createNode(key: string = "", type: ValueType = "string"): TreeNode {
    return {
        id: generateId(),
        key,
        type,
        value: type === "boolean" ? "false" : type === "number" ? "0" : "",
        children: [],
        expanded: true,
    }
}

function valueToTree(value: unknown, key: string = ""): TreeNode {
    const id = generateId()

    if (value === null || value === undefined) {
        return { id, key, type: "string", value: "", children: [], expanded: true }
    }
    if (typeof value === "boolean") {
        return {
            id,
            key,
            type: "boolean",
            value: value ? "true" : "false",
            children: [],
            expanded: true,
        }
    }
    if (typeof value === "number") {
        return { id, key, type: "number", value: String(value), children: [], expanded: true }
    }
    if (typeof value === "string") {
        return { id, key, type: "string", value, children: [], expanded: true }
    }
    if (Array.isArray(value)) {
        return {
            id,
            key,
            type: "array",
            value: "",
            children: value.map((item, index) => valueToTree(item, String(index))),
            expanded: true,
        }
    }
    if (typeof value === "object") {
        return {
            id,
            key,
            type: "object",
            value: "",
            children: Object.entries(value).map(([k, v]) => valueToTree(v, k)),
            expanded: true,
        }
    }
    return { id, key, type: "string", value: String(value), children: [], expanded: true }
}

function treeToValue(node: TreeNode): unknown {
    switch (node.type) {
        case "string":
            return node.value
        case "number":
            return Number(node.value) || 0
        case "boolean":
            return node.value === "true"
        case "array":
            return node.children.map((child) => treeToValue(child))
        case "object": {
            const obj: Record<string, unknown> = {}
            for (const child of node.children) {
                if (child.key.trim()) obj[child.key] = treeToValue(child)
            }
            return obj
        }
    }
}

function dataToTree(data: Record<string, unknown>): TreeNode[] {
    return Object.entries(data).map(([key, value]) => valueToTree(value, key))
}

function treeToData(nodes: TreeNode[]): Record<string, unknown> {
    const data: Record<string, unknown> = {}
    for (const node of nodes) {
        if (node.key.trim()) data[node.key] = treeToValue(node)
    }
    return data
}

const TYPE_LABELS: Record<ValueType, string> = {
    string: "String",
    number: "Number",
    boolean: "Boolean",
    object: "Object",
    array: "Array",
}

interface TreeRowProps {
    node: TreeNode
    depth: number
    isArrayItem: boolean
    index?: number
    onUpdate: (id: string, updates: Partial<TreeNode>) => void
    onDelete: (id: string) => void
    onAddChild: (parentId: string) => void
}

function TreeRow({
    node,
    depth,
    isArrayItem,
    index,
    onUpdate,
    onDelete,
    onAddChild,
}: TreeRowProps) {
    const { t } = useTranslation()
    const isContainer = node.type === "object" || node.type === "array"

    const handleTypeChange = (newType: ValueType) => {
        const updates: Partial<TreeNode> = { type: newType }
        if (newType === "object" || newType === "array") {
            updates.value = ""
            updates.children = []
            updates.expanded = true
        } else {
            updates.children = []
            updates.value = newType === "boolean" ? "false" : newType === "number" ? "0" : ""
        }
        onUpdate(node.id, updates)
    }

    const handleUpdate = (updates: Partial<TreeNode>) => {
        onUpdate(node.id, updates)
    }

    return (
        <div>
            {/* Main Row */}
            <div className="flex items-center py-1.5">
                {/* Expand/Collapse for containers */}
                {isContainer && (
                    <button
                        onClick={() => handleUpdate({ expanded: !node.expanded })}
                        className="w-5 h-5 flex items-center justify-center rounded hover:bg-muted cursor-pointer shrink-0 mr-1"
                        aria-label={
                            node.expanded ? t("collapse", "Collapse") : t("expand", "Expand")
                        }
                        aria-expanded={node.expanded}
                    >
                        <ChevronRight
                            className={cn(
                                "h-3.5 w-3.5 text-muted-foreground transition-transform",
                                node.expanded && "rotate-90",
                            )}
                        />
                    </button>
                )}

                {/* Inline connected inputs - rule builder style */}
                <div className="flex items-center flex-1 min-w-0">
                    <div className="inline-flex items-center">
                        {/* Type Selector */}
                        <Select value={node.type} onValueChange={handleTypeChange}>
                            <SelectTrigger className="h-8 w-[80px] px-2 text-xs font-medium rounded-r-none bg-muted/50 focus:z-10 shadow-none">
                                <SelectValue>{TYPE_LABELS[node.type]}</SelectValue>
                            </SelectTrigger>
                            <SelectContent>
                                {Object.entries(TYPE_LABELS).map(([type, label]) => (
                                    <SelectItem key={type} value={type}>
                                        {label}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>

                        {/* Key */}
                        {isArrayItem ? (
                            <div className="h-8 px-3 flex items-center border border-l-0 bg-muted/30 text-sm text-muted-foreground font-mono -ml-px">
                                [{index}]
                            </div>
                        ) : (
                            <Input
                                value={node.key}
                                onChange={(e) => handleUpdate({ key: e.target.value })}
                                placeholder="key"
                                className="h-8 w-28 rounded-none border-l-0 font-mono text-sm focus:z-10 -ml-px shadow-none"
                            />
                        )}

                        {/* Value section */}
                        {isContainer ? (
                            <>
                                <div className="h-8 px-3 flex items-center border border-l-0 bg-muted/30 text-sm text-muted-foreground -ml-px">
                                    {node.type === "array"
                                        ? `${node.children.length} item${node.children.length !== 1 ? "s" : ""}`
                                        : `${node.children.length} field${node.children.length !== 1 ? "s" : ""}`}
                                </div>
                            </>
                        ) : (
                            <>
                                {/* Equals separator */}
                                <div className="h-8 px-2 flex items-center border border-l-0 bg-muted/50 text-muted-foreground text-sm -ml-px">
                                    =
                                </div>

                                {/* Value input */}
                                {node.type === "boolean" ? (
                                    <Select
                                        value={node.value}
                                        onValueChange={(value) => handleUpdate({ value })}
                                    >
                                        <SelectTrigger className="h-8 w-20 rounded-none border-l-0 text-sm focus:z-10 -ml-px shadow-none">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="true">true</SelectItem>
                                            <SelectItem value="false">false</SelectItem>
                                        </SelectContent>
                                    </Select>
                                ) : (
                                    <Input
                                        value={node.value}
                                        onChange={(e) => handleUpdate({ value: e.target.value })}
                                        placeholder={node.type === "number" ? "0" : "value"}
                                        type={node.type === "number" ? "number" : "text"}
                                        className="h-8 rounded-none border-l-0 text-sm flex-1 min-w-[120px] focus:z-10 -ml-px shadow-none"
                                    />
                                )}
                            </>
                        )}

                        {/* Delete button - integrated into the row */}
                        <button
                            onClick={() => onDelete(node.id)}
                            className="h-8 w-8 flex items-center justify-center border border-l-0 rounded-r-md bg-background text-muted-foreground/60 hover:text-destructive hover:bg-destructive/5 transition-colors -ml-px shrink-0 cursor-pointer"
                            aria-label={t("delete_attribute", "Delete attribute")}
                        >
                            <Trash2 className="h-3.5 w-3.5" />
                        </button>
                    </div>
                </div>
            </div>

            {/* Children - nested inside a pl-6 container to align with type selector */}
            {isContainer && node.expanded && (
                <div className="pl-6">
                    {node.children.map((child, idx) => (
                        <TreeRow
                            key={child.id}
                            node={child}
                            depth={depth + 1}
                            isArrayItem={node.type === "array"}
                            index={idx}
                            onUpdate={onUpdate}
                            onDelete={onDelete}
                            onAddChild={onAddChild}
                        />
                    ))}

                    {/* Add child button */}
                    <div className="flex items-center py-1.5">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => onAddChild(node.id)}
                            className="h-8 text-xs shadow-none"
                        >
                            <Plus className="h-3.5 w-3.5 mr-1.5" />
                            {t("add", "Add")}
                        </Button>
                    </div>
                </div>
            )}
        </div>
    )
}

export interface AttributeEditorProps {
    /** The data object to edit */
    value: Record<string, unknown>
    /** Called when the data changes (only with valid data) */
    onChange: (value: Record<string, unknown>) => void
    /** Optional class name for the container */
    className?: string
    /** Placeholder text for empty state */
    emptyTitle?: string
    /** Description for empty state */
    emptyDescription?: string
    /** Default tab to show */
    defaultTab?: "visual" | "code"
}

export function AttributeEditor({
    value,
    onChange,
    className,
    emptyTitle,
    emptyDescription,
    defaultTab = "visual",
}: AttributeEditorProps) {
    const { t } = useTranslation()

    const [nodes, setNodes] = useState<TreeNode[]>(() => dataToTree(value))
    const [activeTab, setActiveTab] = useState<string>(defaultTab)
    const [codeValue, setCodeValue] = useState<string>(() => JSON.stringify(value, null, 2))
    const [codeError, setCodeError] = useState<string | null>(null)

    // Track if we're making internal changes to avoid sync loops
    const isInternalChange = useRef(false)

    // Debounce timer for code editor changes
    const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

    // Sync state when external value prop changes
    useEffect(() => {
        if (isInternalChange.current) {
            isInternalChange.current = false
            return
        }
        setNodes(dataToTree(value))
        setCodeValue(JSON.stringify(value, null, 2))
    }, [value])

    // Notify parent of changes
    const notifyChange = useCallback(
        (newNodes: TreeNode[]) => {
            isInternalChange.current = true
            const newData = treeToData(newNodes)
            onChange(newData)
        },
        [onChange],
    )

    // Sync code editor when switching tabs
    const handleTabChange = (newTab: string) => {
        if (activeTab === "visual" && newTab === "code") {
            // Switching from visual to code - update code from tree
            const data = treeToData(nodes)
            setCodeValue(JSON.stringify(data, null, 2))
            setCodeError(null)
        } else if (activeTab === "code" && newTab === "visual") {
            // Switching from code to visual - parse code to tree
            try {
                const parsed = JSON.parse(codeValue)
                if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
                    setCodeError(t("json_must_be_object", "JSON must be an object"))
                    return // Don't switch tabs if invalid
                }
                const newNodes = dataToTree(parsed)
                setNodes(newNodes)
                setCodeError(null)
                notifyChange(newNodes)
            } catch {
                setCodeError(t("invalid_json", "Invalid JSON syntax"))
                return // Don't switch tabs if invalid
            }
        }
        setActiveTab(newTab)
    }

    // Handle code editor changes with debouncing
    const handleCodeChange = (newCodeValue: string) => {
        setCodeValue(newCodeValue)

        // Clear existing timer
        if (debounceTimer.current) {
            clearTimeout(debounceTimer.current)
        }

        // Debounce the onChange call to parent
        debounceTimer.current = setTimeout(() => {
            try {
                const parsed = JSON.parse(newCodeValue)
                if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
                    isInternalChange.current = true
                    onChange(parsed)
                }
            } catch {
                // Error handling is done by JsonEditor via onError
            }
        }, 300)
    }

    // Cleanup debounce timer on unmount
    useEffect(() => {
        return () => {
            if (debounceTimer.current) {
                clearTimeout(debounceTimer.current)
            }
        }
    }, [])

    const updateTree = (newNodes: TreeNode[]) => {
        setNodes(newNodes)
        notifyChange(newNodes)
    }

    const findAndUpdate = (
        nodes: TreeNode[],
        id: string,
        updates: Partial<TreeNode>,
    ): TreeNode[] => {
        return nodes.map((node) => {
            if (node.id === id) return { ...node, ...updates }
            if (node.children.length > 0) {
                return { ...node, children: findAndUpdate(node.children, id, updates) }
            }
            return node
        })
    }

    const findAndDelete = (nodes: TreeNode[], id: string): TreeNode[] => {
        return nodes
            .filter((node) => node.id !== id)
            .map((node) => ({ ...node, children: findAndDelete(node.children, id) }))
    }

    const findAndAddChild = (nodes: TreeNode[], parentId: string): TreeNode[] => {
        return nodes.map((node) => {
            if (node.id === parentId) {
                const newChild = createNode(
                    node.type === "array" ? String(node.children.length) : "",
                )
                return { ...node, children: [...node.children, newChild], expanded: true }
            }
            if (node.children.length > 0) {
                return { ...node, children: findAndAddChild(node.children, parentId) }
            }
            return node
        })
    }

    const handleUpdate = (id: string, updates: Partial<TreeNode>) => {
        updateTree(findAndUpdate(nodes, id, updates))
    }

    const handleDelete = (id: string) => {
        updateTree(findAndDelete(nodes, id))
    }

    const handleAddChild = (parentId: string) => {
        updateTree(findAndAddChild(nodes, parentId))
    }

    const addRootNode = () => {
        updateTree([...nodes, createNode()])
    }

    const isEmpty = nodes.length === 0 && activeTab === "visual"

    return (
        <div className={cn("border rounded-lg overflow-hidden", className)}>
            {/* Header with mode toggle */}
            <div className="flex items-center justify-between px-3 py-2 bg-muted/30 border-b">
                <span className="text-sm font-medium text-muted-foreground">
                    {activeTab === "visual"
                        ? t("visual_editor", "Visual Editor")
                        : t("code_editor", "Code Editor")}
                </span>
                <div
                    className="flex items-center rounded-md bg-muted p-0.5"
                    role="tablist"
                    aria-label={t("editor_mode", "Editor mode")}
                >
                    <button
                        onClick={() => handleTabChange("visual")}
                        className={cn(
                            "flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded transition-colors",
                            activeTab === "visual"
                                ? "bg-background text-foreground shadow-sm"
                                : "text-muted-foreground hover:text-foreground",
                        )}
                        role="tab"
                        aria-selected={activeTab === "visual"}
                        aria-controls="visual-panel"
                    >
                        <List className="h-3.5 w-3.5" />
                        {t("visual", "Visual")}
                    </button>
                    <button
                        onClick={() => handleTabChange("code")}
                        className={cn(
                            "flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded transition-colors",
                            activeTab === "code"
                                ? "bg-background text-foreground shadow-sm"
                                : "text-muted-foreground hover:text-foreground",
                        )}
                        role="tab"
                        aria-selected={activeTab === "code"}
                        aria-controls="code-panel"
                    >
                        <Code className="h-3.5 w-3.5" />
                        {t("code", "Code")}
                    </button>
                </div>
            </div>

            {/* Content */}
            {activeTab === "visual" ? (
                isEmpty ? (
                    <div className="flex flex-col items-center justify-center py-12">
                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted mb-3">
                            <Braces className="h-5 w-5 text-muted-foreground" />
                        </div>
                        <p className="font-medium text-sm mb-1">
                            {emptyTitle ?? t("no_attributes", "No attributes yet")}
                        </p>
                        <p className="text-xs text-muted-foreground text-center max-w-xs mb-4">
                            {emptyDescription ??
                                t(
                                    "add_attributes_description",
                                    "Add custom data to store additional information.",
                                )}
                        </p>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={addRootNode}
                            className="shadow-none"
                        >
                            <Plus className="h-4 w-4 mr-2" />
                            {t("add_attribute", "Add attribute")}
                        </Button>
                    </div>
                ) : (
                    <div className="py-2 px-3">
                        {nodes.map((node) => (
                            <TreeRow
                                key={node.id}
                                node={node}
                                depth={0}
                                isArrayItem={false}
                                onUpdate={handleUpdate}
                                onDelete={handleDelete}
                                onAddChild={handleAddChild}
                            />
                        ))}

                        {/* Root add button */}
                        <div className="flex items-center py-1.5">
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={addRootNode}
                                className="h-8 text-xs shadow-none"
                            >
                                <Plus className="h-3.5 w-3.5 mr-1.5" />
                                {t("add", "Add")}
                            </Button>
                        </div>
                    </div>
                )
            ) : (
                <div>
                    <JsonEditor
                        value={codeValue}
                        onChange={handleCodeChange}
                        onError={setCodeError}
                        minHeight={150}
                        maxHeight={400}
                    />
                    {codeError && (
                        <div className="px-3 py-2 text-sm text-destructive bg-destructive/10 border-t">
                            {codeError}
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
