import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { SingleSelect } from "../../../ui/form/SingleSelect";
import type {
  OrganizationRule,
  OrganizationUserMatch,
} from "../../../types";
import {
  operatorTypes,
  userMatchTypes,
  createWrapperRule,
  createInitialMemberCondition,
} from "./RuleHelpers";
import MemberConditionsBuilder from "./MemberConditionsBuilder";

interface OrganizationRuleEditProps {
  rule: OrganizationRule;
  setRule: (rule: OrganizationRule) => void;
  showUserMatch?: boolean; // Whether to show the user match section (for when rendered after children)
}

export default function OrganizationRuleEdit({
  rule,
  setRule,
  showUserMatch = false,
}: OrganizationRuleEditProps) {
  const { t } = useTranslation();

  const userMatch = rule.user_match ?? {
    type: "all",
  };

  // Set default user_match if missing (moved to useEffect to avoid side effects during render)
  useEffect(() => {
    if (!rule.user_match) {
      setRule({
        ...rule,
        user_match: { type: "all" },
      });
    }
  }, [rule, setRule]);

  const handleUserMatchTypeChange = (type: OrganizationUserMatch["type"]) => {
    const newUserMatch: OrganizationUserMatch = { type };

    // Initialize member conditions if conditions type is selected
    if (type === "conditions") {
      // Reuse existing member_conditions or create new wrapper with initial condition
      if (rule.user_match?.member_conditions) {
        newUserMatch.member_conditions = rule.user_match.member_conditions;
      } else {
        const wrapper = createWrapperRule();
        wrapper.children = [createInitialMemberCondition(wrapper)];
        newUserMatch.member_conditions = wrapper;
      }
    }

    setRule({
      ...rule,
      user_match: newUserMatch,
    });
  };

  // When showUserMatch is false, render just the header
  if (!showUserMatch) {
    return (
      <>
        {t("rule_organization_has")}
        {!!rule.children?.length && (
          <>
            {" "}
            {t("rule_matching")}
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
          </>
        )}
      </>
    );
  }

  // When showUserMatch is true, render only the user match section
  return (
    <div className="organization-rule-user-match">
      <div className="user-match-header">
        {t("rule_include_org_members")}
        <SingleSelect
          value={userMatch.type}
          onChange={handleUserMatchTypeChange}
          options={userMatchTypes}
          required
          hideLabel
          size="small"
          toValue={(x) => x.key}
          getOptionDisplay={(x) => x.label}
        />
      </div>

      {/* Member conditions builder when conditions type is selected */}
      {userMatch.type === "conditions" && userMatch.member_conditions && (
        <div className="user-match-conditions">
          <MemberConditionsBuilder
            rule={userMatch.member_conditions}
            setRule={(member_conditions) =>
              setRule({
                ...rule,
                user_match: {
                  ...userMatch,
                  member_conditions,
                },
              })
            }
          />
        </div>
      )}
    </div>
  );
}
