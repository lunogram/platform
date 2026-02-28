import { useContext, useMemo } from "react";
import { highlightSearch, usePopperSelectDropdown } from "../../../ui/utils";
import type { RuleEditProps } from "./RuleHelpers";
import { operatorTypes, VariablesContext, ruleTypes } from "./RuleHelpers";
import { ButtonGroup } from "../../../ui";
import { SingleSelect } from "../../../ui/form/SingleSelect";
import { Combobox } from "../../../components/ui/combobox";
import TextInput from "../../../ui/form/TextInput";
import type { EventSchemaPath, OrganizationSchemaPath, RulePath } from "../../../types";

type PathOption = RulePath | EventSchemaPath | OrganizationSchemaPath;

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
  const isOrganizationEventGroup = group === "organization_event";
  const isOrganizationGroup = group === "organization";

  const pathSuggestions = useMemo<PathOption[]>(() => {
    if (isEventGroup || isOrganizationEventGroup) {
      if (!eventName) return [];
      // Use organization event paths if available for organization events
      const eventSource = isOrganizationEventGroup && suggestions.organizationEventPaths
        ? suggestions.organizationEventPaths
        : suggestions.eventPaths;
      const event = eventSource.find((e) => e.name === eventName);
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

    // Organization property conditions
    if (isOrganizationGroup) {
      let orgPaths = suggestions.organizationPaths ?? [];
      if (path) {
        const search = path.toLowerCase();
        orgPaths = orgPaths.filter((p) =>
          p.path.toLowerCase().includes(search)
        );
      }
      return orgPaths;
    }

    let paths = suggestions.userPaths;
    if (path) {
      let search = path.toLowerCase();
      if (search.startsWith(".")) search = "$" + search;
      if (!search.startsWith("$.")) search = "$." + search;
      paths = paths.filter((p) => p.path.toLowerCase().startsWith(search));
    }
    return paths;
  }, [suggestions, isEventGroup, isOrganizationEventGroup, isOrganizationGroup, eventName, path]);

  const getOptionDataType = (option: PathOption): string => {
    if ("types" in option) {
      return option.types[0] || "string";
    }
  
    return option.data_type;
  };

  return (
    <div className="rule">
      <ButtonGroup className="ui-select">
        <SingleSelect
          value={rule?.type}
          onChange={(type) => setRule({ ...rule, type })}
          options={ruleTypes}
          required
          hideLabel
          size="small"
          toValue={(x) => x.key as typeof rule.type}
        />
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
          value={rule?.operator}
          onChange={(operator) => setRule({ ...rule, operator })}
          options={operatorTypes[rule?.type] ?? []}
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
