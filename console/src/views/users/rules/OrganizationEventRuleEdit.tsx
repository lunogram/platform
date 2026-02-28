import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import RuleOrganizationEventName from "./RuleOrganizationEventName";
import { ButtonGroup } from "../../../ui";
import { SingleSelect } from "../../../ui/form/SingleSelect";
import type {
  OrganizationEventRule,
  OrganizationUserMatch,
} from "../../../types";
import TextInput from "../../../ui/form/TextInput";
import {
  frequencyOperators,
  operatorTypes,
  periodUnits,
  userMatchTypes,
  createWrapperRule,
  createInitialMemberCondition,
} from "./RuleHelpers";
import MemberConditionsBuilder from "./MemberConditionsBuilder";

interface OrganizationEventRuleEditProps {
  rule: OrganizationEventRule;
  eventName?: string;
  setRule: (rule: OrganizationEventRule) => void;
}

export default function OrganizationEventRuleEdit({
  rule,
  setRule,
  eventName,
}: OrganizationEventRuleEditProps) {
  const { t } = useTranslation();

  const defaultFrequency = {
    period: {
      type: "rolling" as const,
      unit: "day" as const,
      value: 30,
    },
    operator: ">=" as const,
    count: 1,
  };

  const frequency = rule.frequency ?? defaultFrequency;

  const userMatch = rule.user_match ?? {
    type: "all" as const,
  };

  // Set default frequency and user_match if missing (moved to useEffect to avoid side effects during render)
  useEffect(() => {
    if (!rule.frequency || !rule.user_match) {
      setRule({
        ...rule,
        frequency: rule.frequency ?? defaultFrequency,
        user_match: rule.user_match ?? { type: "all" },
      });
    }
  }, [rule, setRule]);

  // When editing nested conditions, show a simpler header
  if (eventName) {
    if (rule.children?.length) {
      return (
        <>
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
      );
    }
    return <></>;
  }

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

  return (
    <div className="organization-event-rule">
      <div className="organization-event-rule-header">
        {t("rule_organization_did")}
        <ButtonGroup className="ui-select event-name">
          <span className="ui-select">
            <RuleOrganizationEventName rule={rule} setRule={setRule} />
          </span>
        </ButtonGroup>
        <ButtonGroup className="ui-select frequency-count">
          <SingleSelect
            value={frequency.operator}
            onChange={(operator) =>
              setRule({
                ...rule,
                frequency: {
                  ...(rule.frequency ?? frequency),
                  operator,
                },
              })
            }
            options={frequencyOperators}
            required
            hideLabel
            size="small"
            toValue={(x) => x.key}
          />
          <TextInput
            size="tiny"
            type="text"
            name="value"
            placeholder="Count"
            hideLabel={true}
            value={frequency.count?.toString()}
            onChange={(count) => {
              setRule({
                ...rule,
                frequency: {
                  ...(rule.frequency ?? frequency),
                  count: count ? parseInt(count, 10) : undefined,
                },
              });
            }}
          />
        </ButtonGroup>
        {t("rule_times", "times")}
        {frequency.period.type === "rolling" && (
          <>
            {" "}
            {t("rule_in_last", "in last")}
            <ButtonGroup className="ui-select frequency-period">
              <TextInput
                size="tiny"
                type="text"
                name="value"
                placeholder="Value"
                hideLabel={true}
                value={frequency.period.value.toString()}
                onChange={(value) => {
                  if (frequency.period.type !== "rolling") return;
                  setRule({
                    ...rule,
                    frequency: {
                      ...frequency,
                      period: {
                        ...frequency.period,
                        value: parseInt(value, 10) || 1,
                      },
                    },
                  });
                }}
              />
              <SingleSelect
                value={frequency.period.unit}
                onChange={(unit) => {
                  if (frequency.period.type !== "rolling") return;
                  setRule({
                    ...rule,
                    frequency: {
                      ...frequency,
                      period: {
                        ...frequency.period,
                        unit,
                      },
                    },
                  });
                }}
                options={periodUnits}
                required
                hideLabel
                size="small"
                toValue={(x) => x.key}
              />
            </ButtonGroup>
          </>
        )}
      </div>

      {/* User matching section */}
      <div className="organization-event-rule-user-match">
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

      {/* Event conditions section */}
      {!!rule.children?.length && (
        <div className="organization-event-rule-conditions">
          {t("rule_event_conditions_matching")}
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
      )}
    </div>
  );
}
