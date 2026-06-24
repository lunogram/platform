import { Plus, Trash2 } from "lucide-react"
import { Input } from "@/components/ui/input"
import { TemplateInput } from "@/components/ui/template-input"
import { Button } from "@/components/ui/button"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

interface KeyValuePair {
    key: string
    value: string
}

interface KeyValueEditorProps {
    value: Record<string, string>
    onChange: (value: Record<string, string>) => void
    keyPlaceholder?: string
    valuePlaceholder?: string
    /** Label for the add-row button (defaults to "Add Row"). */
    addLabel?: string
    /** Optional variable groups for autocomplete in the value field. */
    variables?: VariableGroup[]
}

function toRows(obj: Record<string, string>): KeyValuePair[] {
    const entries = Object.entries(obj)
    return entries.length > 0
        ? entries.map(([key, value]) => ({ key, value }))
        : [{ key: "", value: "" }]
}

function toRecord(rows: KeyValuePair[]): Record<string, string> {
    const result: Record<string, string> = {}
    for (const row of rows) {
        const k = row.key.trim()
        if (k) result[k] = row.value
    }
    return result
}

export function KeyValueEditor({
    value,
    onChange,
    keyPlaceholder = "Key",
    valuePlaceholder = "Value",
    addLabel = "Add Row",
    variables,
}: KeyValueEditorProps) {
    const rows = toRows(value ?? {})
    const hasVariables = variables && variables.some((g) => g.variables.length > 0)

    function update(next: KeyValuePair[]) {
        onChange(toRecord(next))
    }

    function setRow(index: number, field: "key" | "value", val: string) {
        const next = [...rows]
        next[index] = { ...next[index], [field]: val }
        update(next)
    }

    function addRow() {
        update([...rows, { key: "", value: "" }])
    }

    function removeRow(index: number) {
        const next = rows.filter((_, i) => i !== index)
        update(next.length > 0 ? next : [{ key: "", value: "" }])
    }

    return (
        <div className="space-y-2">
            {rows.map((row, i) => (
                <div key={i} className="flex items-center gap-2">
                    <Input
                        value={row.key}
                        onChange={(e) => setRow(i, "key", e.target.value)}
                        placeholder={keyPlaceholder}
                        className="flex-1"
                    />
                    {hasVariables ? (
                        <TemplateInput
                            value={row.value}
                            onChange={(val) => setRow(i, "value", val)}
                            placeholder={valuePlaceholder}
                            className="flex-1"
                            variables={variables}
                        />
                    ) : (
                        <Input
                            value={row.value}
                            onChange={(e) => setRow(i, "value", e.target.value)}
                            placeholder={valuePlaceholder}
                            className="flex-1"
                        />
                    )}
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="shrink-0 text-muted-foreground hover:text-destructive"
                        onClick={() => removeRow(i)}
                        aria-label={`Remove row ${i + 1}`}
                    >
                        <Trash2 className="h-4 w-4" />
                    </Button>
                </div>
            ))}
            <Button type="button" variant="outline" size="sm" onClick={addRow} className="mt-1">
                <Plus className="h-3.5 w-3.5 mr-1.5" />
                {addLabel}
            </Button>
        </div>
    )
}
