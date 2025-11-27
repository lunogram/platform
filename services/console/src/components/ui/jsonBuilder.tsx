import { useEffect, useRef, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "./card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "./dropdown-menu";
import { Button } from "./button";
import {
  EllipsisVertical,
  Plus,
  Trash2,
  ListPlus,
  Code,
  LayoutGrid,
} from "lucide-react";
import { Separator } from "./separator";
import { Badge } from "@/components/ui/badge";
import { Input } from "./input";
import { FieldSet } from "./field";
import { Textarea } from "./textarea";

type FieldType = "string" | "number" | "array" | "time" | "null";

type Field = {
  id: string;
  key: string;
  type: FieldType;
  value?: string | number | string[] | Date | null;
};

export function JsonBuilderField({
  onChange,
}: {
  onChange?: (json: Record<string, unknown>) => void;
}) {
  const [json, setJson] = useState({});
  const [fields, setFields] = useState<Field[]>([]);
  const [mode, setMode] = useState<"visual" | "json">("visual");
  const [jsonText, setJsonText] = useState("{}");
  const [jsonError, setJsonError] = useState<string | null>(null);
  const scrollAreaRef = useRef<HTMLDivElement>(null);

  const addField = (type: FieldType) => {
    let defaultValue: Field["value"];
    switch (type) {
      case "null":
        defaultValue = null;
        break;
      case "time":
        defaultValue = new Date();
        break;
      case "number":
        defaultValue = 0;
        break;
      case "array":
        defaultValue = [""];
        break;
      case "string":
        defaultValue = "";
        break;
      default:
        defaultValue = undefined;
    }

    const newField: Field = {
      id: crypto.randomUUID(),
      key: "",
      type,
      value: defaultValue,
    };
    setFields([...fields, newField]);
  };

  const updateField = (id: string, patch: Partial<Field>) => {
    setFields((prev) =>
      prev.map((f) => (f.id === id ? { ...f, ...patch } : f))
    );
  };

  const removeField = (id: string) => {
    setFields((prev) => prev.filter((f) => f.id !== id));
  };

  useEffect(() => {
    // Rebuild JSON from fields
    const newJson = fields.reduce(
      (acc, field) => {
        if (field.key) {
          // Serialize time as ISO string
          if (field.type === "time" && field.value instanceof Date) {
            acc[field.key] = field.value.toISOString();
          } else if (field.type === "number") {
            acc[field.key] =
              typeof field.value === "number"
                ? field.value
                : Number(field.value);
          } else {
            acc[field.key] = field.value;
          }
        }
        return acc;
      },
      {} as Record<string, unknown>
    );
    setJson(newJson);
    setJsonText(JSON.stringify(newJson, null, 2));
  }, [fields]);

  const handleJsonTextChange = (text: string) => {
    setJsonText(text);
    try {
      const parsed = JSON.parse(text);
      setJsonError(null);
      setJson(parsed);

      // Convert parsed JSON back to fields
      const newFields: Field[] = Object.entries(parsed).map(([key, value]) => {
        let type: FieldType = "string";
        let fieldValue: Field["value"] = value as Field["value"];

        if (value === null) {
          type = "null";
        } else if (Array.isArray(value)) {
          type = "array";
          fieldValue = value.map((v) => String(v));
        } else if (typeof value === "number") {
          type = "number";
        } else if (typeof value === "string") {
          // Check if it's an ISO date string
          const dateRegex = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/;
          if (dateRegex.test(value)) {
            type = "time";
            fieldValue = new Date(value);
          } else {
            type = "string";
          }
        }

        return {
          id: crypto.randomUUID(),
          key,
          type,
          value: fieldValue,
        };
      });

      setFields(newFields);
    } catch (error) {
      setJsonError(error instanceof Error ? error.message : "Invalid JSON");
    }
  };

  useEffect(() => {
    onChange?.(json);
  }, [json, onChange]);

  useEffect(() => {
    // Scroll to bottom when fields are added
    if (scrollAreaRef.current) {
      requestAnimationFrame(() => {
        if (scrollAreaRef.current) {
          scrollAreaRef.current.scrollTop = scrollAreaRef.current.scrollHeight;
        }
      });
    }
  }, [fields.length]);

  return (
    <>
      <Card className="mb-4">
        <CardHeader className="flex flex-row justify-between items-center">
          <CardTitle>Fields</CardTitle>
          <div className="flex gap-2">
            {mode === "visual" && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="sm" type="button">
                    <Plus className="h-4 w-4 mr-2" />
                    Add Field
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-40">
                  <DropdownMenuItem onClick={() => addField("string")}>
                    String
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => addField("number")}>
                    Number
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => addField("array")}>
                    Array
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => addField("time")}>
                    Time
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => addField("null")}>
                    Null
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
            <Button
              size="sm"
              variant={mode === "visual" ? "default" : "outline"}
              type="button"
              onClick={() => setMode("visual")}
            >
              <LayoutGrid className="h-4 w-4 mr-2" />
              Visual
            </Button>
            <Button
              size="sm"
              variant={mode === "json" ? "default" : "outline"}
              type="button"
              onClick={() => setMode("json")}
            >
              <Code className="h-4 w-4 mr-2" />
              JSON
            </Button>
          </div>
        </CardHeader>
        <Separator className="my-2" />
        <CardContent className="space-y-4">
          {mode === "visual" ? (
            fields.length === 0 ? (
              <p className="text-sm text-muted-foreground mt-5">
                No fields added. Use the "Add Field" button to add fields. Or
                switch to JSON mode to edit the JSON directly for more complex
                structures.
              </p>
            ) : (
              <div className="max-h-[420px] overflow-auto" ref={scrollAreaRef}>
                <div className="space-y-4 pr-4">
                  {fields.map((field) => (
                    <FieldRow
                      key={field.id}
                      field={field}
                      onChange={(patch) => updateField(field.id, patch)}
                      onRemove={() => removeField(field.id)}
                    />
                  ))}
                </div>
              </div>
            )
          ) : (
            <div className="space-y-2">
              <Textarea
                value={jsonText}
                onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                  handleJsonTextChange(e.target.value)
                }
                placeholder="Enter JSON here..."
                className="font-mono min-h-[400px]"
              />
              {jsonError && (
                <p className="text-sm text-red-600">Error: {jsonError}</p>
              )}
              {/* TODO: Change this field to the field used in Campaigns! */}
            </div>
          )}
        </CardContent>
      </Card>
    </>
  );
}

function FieldRow({
  field,
  onChange,
  onRemove,
}: {
  field: Field;
  onChange: (patch: Partial<Field>) => void;
  onRemove: () => void;
}) {
  return (
    <>
      <FieldSet className="gap-2">
        <div className="flex flex-row items-center mt-2 gap-2">
          <Badge variant="secondary">{field.type}</Badge>
          <Input
            className="w-48"
            placeholder="key"
            value={field.key}
            onChange={(e) => onChange({ key: e.target.value })}
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" type="button">
                <EllipsisVertical className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40 z-999">
              <DropdownMenuItem onClick={onRemove} className="text-red-600">
                <Trash2 className="h-4 w-4 mr-2" /> Remove
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <div>
          {field.type === "string" && (
            <Input
              placeholder="String value"
              value={typeof field.value === "string" ? field.value : ""}
              onChange={(e) => onChange({ value: e.target.value })}
            />
          )}
          {field.type === "number" && (
            <Input
              type="number"
              placeholder="Number value"
              value={typeof field.value === "number" ? field.value : ""}
              onChange={(e) => onChange({ value: Number(e.target.value) || 0 })}
            />
          )}
          {field.type === "time" && (
            <Input
              type="datetime-local"
              value={
                field.value instanceof Date
                  ? field.value.toISOString().slice(0, 16)
                  : ""
              }
              onChange={(e) => onChange({ value: new Date(e.target.value) })}
            />
          )}
          {field.type === "null" && (
            <div className="text-sm text-muted-foreground">
              Value: <Badge>null</Badge>
            </div>
          )}
          {field.type === "array" && (
            <div className="grid gap-2">
              {(Array.isArray(field.value) ? field.value : [""]).map(
                (v, idx) => (
                  <div key={idx} className="flex items-center gap-2">
                    <Input
                      placeholder={`Item ${idx + 1}`}
                      value={v}
                      onChange={(e) => {
                        const currentArray = Array.isArray(field.value)
                          ? field.value
                          : [""];
                        const next = [...currentArray];
                        next[idx] = e.target.value;
                        onChange({ value: next });
                      }}
                    />
                    <Button
                      variant="ghost"
                      size="icon"
                      type="button"
                      onClick={() => {
                        const currentArray = Array.isArray(field.value)
                          ? field.value
                          : [""];
                        const next = currentArray.filter((_, i) => i !== idx);
                        onChange({ value: next.length ? next : [""] });
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                )
              )}
              <Button
                className="mt-1 w-fit gap-2"
                variant="secondary"
                size="sm"
                type="button"
                onClick={() => {
                  const currentArray = Array.isArray(field.value)
                    ? field.value
                    : [""];
                  onChange({ value: [...currentArray, ""] });
                }}
              >
                <ListPlus className="h-4 w-4" /> Add Item
              </Button>
            </div>
          )}
        </div>
      </FieldSet>
    </>
  );
}
