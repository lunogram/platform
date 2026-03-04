import { Button } from "@/components/ui/button";
import { ButtonGroup } from "../../../ui";
import { SingleSelect } from "../../../ui/form/SingleSelect";
import { Combobox } from "../../../components/ui/combobox";
import TextInput from "../../../ui/form/TextInput";
import { TrashIcon } from "../../../components/icons";
import type { OrganizationUserSchemaPath, Rule, RuleType } from "../../../types";
import { operatorTypes, ruleTypes } from "./RuleHelpers";
import { highlightSearch } from "../../../ui/utils";

interface MemberConditionEditProps {
  rule: Rule;
  setRule: (rule: Rule) => void;
  onRemove: () => void;
  organizationUserPaths: OrganizationUserSchemaPath[];
}

// Map schema types to rule types
function mapSchemaTypeToRuleType(types: string[]): RuleType {
  // Use the first type as the primary type
  const type = types[0]?.toLowerCase();
  switch (type) {
    case "string":
      return "string";
    case "number":
    case "integer":
    case "float":
      return "number";
    case "boolean":
    case "bool":
      return "boolean";
    case "date":
    case "datetime":
    case "timestamp":
      return "date";
    case "array":
    case "list":
      return "array";
    default:
      return "string";
  }
}

/**
 * Edit a single member property condition.
 * This is for filtering organization members based on their membership data properties.
 */
export default function MemberConditionEdit({
  rule,
  setRule,
  onRemove,
  organizationUserPaths,
}: MemberConditionEditProps) {
  const hasValue =
    rule?.operator &&
    !["is set", "is not set", "empty"].includes(rule?.operator);

  // Convert schema paths to combobox options
  // API returns paths like ".role", ".admin" - use them as-is without adding .data prefix
  const pathOptions = organizationUserPaths.map((p) => {
    const dataType = mapSchemaTypeToRuleType(p.types);
    // Extract name from path (e.g., ".role" -> "role")
    const name = p.path.startsWith(".") ? p.path.substring(1) : p.path;
    return {
      id: `member-path-${p.path}`,
      path: p.path,
      name,
      type: "user" as const,
      data_type: dataType,
      visibility: "public" as const,
    };
  });

  return (
    <div className="member-condition">
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
            const suggestion = pathOptions.find(
              (s) => s.path === selectedPath
            );
            if (suggestion) {
              setRule({
                ...rule,
                type: suggestion.data_type as typeof rule.type,
                path: suggestion.path,
              });
            } else {
              setRule({ ...rule, path: selectedPath });
            }
          }}
          options={pathOptions}
          placeholder="Member property path"
          required
          inputClassName="rounded-none border-l-0"
          buttonClassName="rounded-none"
          renderOption={(option, search) => (
            <span
              dangerouslySetInnerHTML={{
                __html: highlightSearch(option.name || option.path, search),
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
          hasValue && (
            <TextInput
              size="small"
              type="text"
              name="value"
              placeholder="Value"
              hideLabel={true}
              value={rule?.value?.toString()}
              onChange={(value) => setRule({ ...rule, value })}
            />
          )
        )}
        <Button
          size="sm"
          variant="outline"
          className="rounded-l-none shadow-none border-l-0"
          onClick={onRemove}
        >
          <TrashIcon />
        </Button>
      </ButtonGroup>
    </div>
  );
}
