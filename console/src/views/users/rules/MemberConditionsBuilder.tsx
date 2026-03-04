import { useContext } from "react";
import { Button } from "@/components/ui/button";
import { SingleSelect } from "../../../ui/form/SingleSelect";
import { PlusIcon } from "../../../components/icons";
import { createUuid } from "../../../utils";
import type { Rule, WrapperRule } from "../../../types";
import { operatorTypes, VariablesContext } from "./RuleHelpers";
import MemberConditionEdit from "./MemberConditionEdit";
import { useTranslation } from "react-i18next";

interface MemberConditionsBuilderProps {
  rule: WrapperRule;
  setRule: (rule: WrapperRule) => void;
}

/**
 * A simplified rule builder specifically for organization member property conditions.
 * This only supports property-based conditions on member data, not events.
 */
export default function MemberConditionsBuilder({
  rule,
  setRule,
}: MemberConditionsBuilderProps) {
  const { t } = useTranslation();
  const { suggestions } = useContext(VariablesContext);

  const handleAddCondition = () => {
    const newCondition: Rule = {
      uuid: createUuid(),
      root_uuid: rule.uuid,
      parent_uuid: rule.uuid,
      path: "",
      type: "string",
      group: "user", // member data is treated like user properties
      value: "",
      operator: "=",
    };

    setRule({
      ...rule,
      children: [...(rule.children ?? []), newCondition],
    });
  };

  const handleRemoveCondition = (index: number) => {
    setRule({
      ...rule,
      children: rule.children?.filter((_, i) => i !== index) ?? [],
    });
  };

  const handleUpdateCondition = (index: number, updatedCondition: Rule) => {
    setRule({
      ...rule,
      children: rule.children?.map((c, i) =>
        i === index ? updatedCondition : c
      ) ?? [],
    });
  };

  return (
    <div className="member-conditions-builder">
      <div className="member-conditions-header">
        {t("rule_members_matching")}
        <SingleSelect
          value={rule.operator}
          onChange={(operator) => setRule({ ...rule, operator })}
          options={operatorTypes.wrapper}
          required
          hideLabel
          size="small"
          toValue={(x) => x.key}
        />
        {t("rule_of_the_following")}
      </div>

      <div className="member-conditions-list">
        {rule.children?.map((child, index) => (
          <MemberConditionEdit
            key={child.uuid || index}
            rule={child}
            setRule={(updated) => handleUpdateCondition(index, updated)}
            onRemove={() => handleRemoveCondition(index)}
            organizationUserPaths={suggestions.organizationUserPaths ?? []}
          />
        ))}
      </div>

      <div className="member-conditions-actions">
        <Button
          size="sm"
          variant="outline"
          onClick={handleAddCondition}
        >
          <PlusIcon />
          {t("rule_add_member_condition")}
        </Button>
      </div>
    </div>
  );
}
