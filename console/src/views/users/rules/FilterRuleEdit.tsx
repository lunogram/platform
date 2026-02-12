import { useContext, useMemo } from "react";
import { highlightSearch, usePopperSelectDropdown } from "../../../ui/utils";
import type { RuleEditProps } from "./RuleHelpers";
import { operatorTypes, VariablesContext, ruleTypes } from "./RuleHelpers";
import { Combobox } from "@/components/ui/combobox";
import type { EventSchemaPath, RulePath } from "../../../types";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";

type PathOption = RulePath | EventSchemaPath;

export default function FilterRuleEdit({
  rule,
  setRule,
  group,
  eventName = "",
  controls,
}: Omit<RuleEditProps, "root" | "headerPrefix" | "depth">) {
  usePopperSelectDropdown();
  const { suggestions } = useContext(VariablesContext);
  const { path } = rule ?? {};
  const hasValue =
    rule?.operator &&
    !["is set", "is not set", "empty"].includes(rule?.operator);

  const isEventGroup = group === "event";

  const pathSuggestions = useMemo<PathOption[]>(() => {
    if (isEventGroup) {
      if (!eventName) return [];
      const event = suggestions.eventPaths.find((e) => e.name === eventName);
      if (!event) return [];
      let schemaPaths = event.schema;
      if (path) {
        const search = path.toLowerCase();
        schemaPaths = schemaPaths.filter((s) =>
          s.path.toLowerCase().includes(search)
        );
      }
      return schemaPaths;
    }

    let paths = suggestions.userPaths;
    if (path) {
      let search = path.toLowerCase();
      if (search.startsWith(".")) search = "$" + search;
      if (!search.startsWith("$.")) search = "$" + search;
      paths = paths.filter((p) => p.path.toLowerCase().startsWith(search));
    }
    return paths;
  }, [suggestions, isEventGroup, eventName, path]);

  const getOptionDataType = (option: PathOption): string => {
    if ("types" in option) {
      return option.types[0] || "string";
    }

    return option.data_type;
  };

  const typeOption = ruleTypes.find((opt) => opt.key === rule?.type);
  const operatorOption = operatorTypes[rule?.type]?.find(
    (opt) => opt.key === rule?.operator
  );

  return (
    <div className="rule">
      <div className="inline-flex items-stretch">
        <Select
          value={rule?.type}
          onValueChange={(type) =>
            setRule({ ...rule, type: type as typeof rule.type })
          }
        >
          <SelectTrigger className="h-9 w-auto rounded-r-none border-r-0 text-sm">
            <SelectValue placeholder={typeOption?.label}>
              {typeOption?.label}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {ruleTypes.map((opt) => (
              <SelectItem key={opt.key} value={opt.key}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Combobox
          value={rule?.path}
          onValueChange={(selectedPath: string) => {
            const suggestion = pathSuggestions.find(
              (s) => s.path === selectedPath
            );
            if (suggestion) {
              setRule({
                ...rule,
                type: getOptionDataType(suggestion) as typeof rule.type,
                path: suggestion.path,
              });
            } else {
              setRule({ ...rule, path: selectedPath });
            }
          }}
          options={pathSuggestions}
          placeholder="Path"
          required
          inputClassName="rounded-none"
          buttonClassName="rounded-none"
          renderOption={(option, search) => (
            <span
              dangerouslySetInnerHTML={{
                __html: highlightSearch(option.path, search),
              }}
            />
          )}
        />
        <Select
          value={rule?.operator}
          onValueChange={(operator) =>
            setRule({ ...rule, operator: operator as typeof rule.operator })
          }
        >
          <SelectTrigger className="h-9 w-auto rounded-none border-x-0 text-sm">
            <SelectValue placeholder={operatorOption?.label}>
              {operatorOption?.label}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {(operatorTypes[rule?.type] ?? []).map((opt) => (
              <SelectItem key={opt.key} value={opt.key}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {hasValue && rule.type === "boolean" ? (
          <Select
            value={
              rule.value === "true"
                ? "true"
                : rule.value === "false"
                  ? "false"
                  : undefined
            }
            onValueChange={(value) => setRule({ ...rule, value })}
          >
            <SelectTrigger className="h-9 w-auto rounded-l-none text-sm">
              <SelectValue placeholder="Value" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="true">True</SelectItem>
              <SelectItem value="false">False</SelectItem>
            </SelectContent>
          </Select>
        ) : (
          <Input
            type="text"
            placeholder="Value"
            value={rule?.value?.toString() ?? ""}
            onChange={(e) => setRule({ ...rule, value: e.target.value })}
            className="h-9 w-auto rounded-l-none border-l-0"
          />
        )}
        {controls}
      </div>
    </div>
  );
}
