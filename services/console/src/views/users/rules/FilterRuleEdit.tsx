import { useContext, useMemo } from "react";
import { highlightSearch, usePopperSelectDropdown } from "../../../ui/utils";
import type { RuleEditProps } from "./RuleHelpers";
import { operatorTypes, VariablesContext, ruleTypes } from "./RuleHelpers";
import { ButtonGroup } from "../../../ui";
import { SingleSelect } from "../../../ui/form/SingleSelect";
import { Combobox } from "../../../components/ui/combobox";
import TextInput from "../../../ui/form/TextInput";
import type { RulePath } from "../../../types";

export default function FilterRuleEdit({
  rule,
  setRule,
  group,
  eventName = "",
  controls,
}: Omit<RuleEditProps, "root" | "headerPrefix" | "depth">) {
  usePopperSelectDropdown();
  const { suggestions } = useContext(VariablesContext);
  const { path } = rule;
  const hasValue =
    rule?.operator &&
    !["is set", "is not set", "empty"].includes(rule?.operator);
  const pathSuggestions = useMemo<RulePath[]>(() => {
    let paths =
      group === "event"
        ? eventName
          ? (suggestions.eventPaths[eventName] ?? [])
          : []
        : suggestions.userPaths;

    if (path) {
      let search = path.toLowerCase();
      if (search.startsWith(".")) search = "$" + search;
      if (!search.startsWith("$.")) search = "$." + search;
      paths = paths.filter((p) => p.path.toLowerCase().startsWith(search));
    }

    return paths;
  }, [suggestions, group, eventName, path]);
  const dummyPathSuggestions: RulePath[] = [
    {
      id: "1",
      path: "$.user.email",
      name: "User Email",
      type: "user",
      data_type: "string",
      visibility: "public",
    },
    {
      id: "2",
      path: "$.user.name",
      name: "User Name",
      type: "user",
      data_type: "string",
      visibility: "public",
    },
    {
      id: "3",
      path: "$.user.age",
      name: "User Age",
      type: "user",
      data_type: "number",
      visibility: "public",
    },
  ];

  return (
    <div className="rule">
      <ButtonGroup className="ui-select">
        <SingleSelect
          value={rule.type}
          onChange={(type) => setRule({ ...rule, type })}
          options={ruleTypes}
          required
          hideLabel
          size="small"
          toValue={(x) => x.key as typeof rule.type}
        />
        <Combobox
          value={rule.path}
          onValueChange={(selectedPath: string) => {
            const suggestion = dummyPathSuggestions.find(
              (s) => s.path === selectedPath
            );
            if (suggestion) {
              setRule({
                ...rule,
                type: suggestion.data_type,
                path: suggestion.path,
              });
            } else {
              setRule({ ...rule, path: selectedPath });
            }
          }}
          options={dummyPathSuggestions}
          placeholder="Path"
          required
          inputClassName="rounded-none border-l-0"
          buttonClassName="rounded-none"
          renderOption={(option, search) => (
            <span
              dangerouslySetInnerHTML={{
                __html: highlightSearch(option.path, search),
              }}
            />
          )}
        />
        <SingleSelect
          value={rule.operator}
          onChange={(operator) => setRule({ ...rule, operator })}
          options={operatorTypes[rule.type] ?? []}
          required
          hideLabel
          size="small"
          toValue={(x) => x.key}
        />
        {hasValue && rule.type === "boolean" ? (
          <SingleSelect
            value={
              rule.value === "true"
                ? "true"
                : rule.value === "false"
                  ? "false"
                  : undefined
            }
            onChange={(value) => setRule({ ...rule, value })}
            options={[
              { key: "true", label: "True" },
              { key: "false", label: "False" },
            ]}
            required
            hideLabel
            size="small"
            toValue={(x) => x.key}
          />
        ) : (
          <TextInput
            size="small"
            type="text"
            name="value"
            placeholder="Value"
            hideLabel={true}
            value={rule?.value?.toString()}
            onChange={(value) => setRule({ ...rule, value })}
          />
        )}
        {controls}
      </ButtonGroup>
    </div>
  );
}
