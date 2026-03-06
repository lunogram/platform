import { useTranslation } from "react-i18next"
import type { UseFormReturn } from "react-hook-form"

import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

import type { ActionFormValues } from "./action-form-types"

interface TestVariablesDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    form: UseFormReturn<ActionFormValues>
    testVariableValues: Record<string, string>
    setTestVariableValues: React.Dispatch<React.SetStateAction<Record<string, string>>>
    isTesting: boolean
    onRunTest: (overrides: Record<string, string>) => void
}

export function TestVariablesDialog({
    open,
    onOpenChange,
    form,
    testVariableValues,
    setTestVariableValues,
    isTesting,
    onRunTest,
}: TestVariablesDialogProps) {
    const { t } = useTranslation()

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("test_variables", "Test Variables")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "test_variables_description",
                            "Provide values for the variables used in this action. These values will replace {{variable}} placeholders during the test.",
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-3 py-2">
                    {(form.getValues("variables") ?? [])
                        .filter((v) => v.name.trim())
                        .map((v) => {
                            const key = v.name.trim()
                            return (
                                <Field key={key} className="gap-1.5">
                                    <FieldLabel className="flex items-center gap-2">
                                        <span className="font-mono text-xs">{key}</span>
                                        <Badge variant="outline" className="text-[10px] px-1 py-0">
                                            {v.type}
                                        </Badge>
                                    </FieldLabel>
                                    {v.type === "bool" ? (
                                        <Select
                                            value={testVariableValues[key] ?? ""}
                                            onValueChange={(val) =>
                                                setTestVariableValues((prev) => ({
                                                    ...prev,
                                                    [key]: val,
                                                }))
                                            }
                                        >
                                            <SelectTrigger className="text-sm">
                                                <SelectValue
                                                    placeholder={t("select_value", "Select value")}
                                                />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="true">true</SelectItem>
                                                <SelectItem value="false">false</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    ) : (
                                        <Input
                                            type={v.type === "int" ? "number" : "text"}
                                            value={testVariableValues[key] ?? ""}
                                            onChange={(e) =>
                                                setTestVariableValues((prev) => ({
                                                    ...prev,
                                                    [key]: e.target.value,
                                                }))
                                            }
                                            placeholder={v.default || key}
                                            className="text-sm"
                                            autoComplete="off"
                                        />
                                    )}
                                </Field>
                            )
                        })}
                </div>
                <DialogFooter>
                    <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                        {t("cancel")}
                    </Button>
                    <Button
                        type="button"
                        disabled={isTesting}
                        isLoading={isTesting}
                        onClick={() => {
                            onOpenChange(false)
                            onRunTest(testVariableValues)
                        }}
                    >
                        {t("run_test", "Run Test")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
