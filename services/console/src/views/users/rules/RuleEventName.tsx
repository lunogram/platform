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
  const dummySuggestions: RulePath[] = [
    {
      id: "1",
      path: "user.signup",
      name: "User Signup",
      type: "event",
      data_type: "string",
      visibility: "public",
    },
    {
      id: "2",
      path: "user.login",
      name: "User Login",
      type: "event",
      data_type: "string",
      visibility: "public",
    },
    {
      id: "3",
      path: "purchase.completed",
      name: "Purchase Completed",
      type: "event",
      data_type: "string",
      visibility: "public",
    },
  ];
  return (
    <Combobox
      value={rule.value ?? ''}
      onValueChange={(selectedPath: string) => {
        setRule({ ...rule, value: selectedPath });
      }}
      options={dummySuggestions}
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
