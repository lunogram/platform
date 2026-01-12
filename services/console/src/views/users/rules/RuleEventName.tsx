import { Combobox } from "../../../components/ui/combobox";
import type { Rule, RulePath } from "../../../types";
import { highlightSearch, usePopperSelectDropdown } from "../../../ui/utils";
import { useContext } from "react";
import { VariablesContext } from "./RuleHelpers";

export default function RuleEventName<T extends Rule>({
  rule,
  setRule,
}: {
  rule: T;
  setRule: (rule: T) => void;
}) {
  usePopperSelectDropdown();

  const { suggestions } = useContext(VariablesContext);
  
  // Convert event names (keys) to RulePath objects for the Combobox
  const eventOptions: RulePath[] = Object.keys(suggestions.eventPaths ?? {})
    .sort()
    .map((eventName, index) => ({
      id: `event-${index}`,
      path: eventName,
      name: eventName,
      type: "event" as const,
      data_type: "string" as const,
      visibility: "public" as const,
    }));

  return (
    <Combobox
      value={rule.value ?? ''}
      onValueChange={(selectedPath: string) => {
        setRule({ ...rule, value: selectedPath });
      }}
      options={eventOptions}
      placeholder="Event name"
      required
      renderOption={(option, search) => (
        <span
          dangerouslySetInnerHTML={{
            __html: highlightSearch(option.path, search),
          }}
        />
      )}
    />
  );
}
