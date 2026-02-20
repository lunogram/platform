import { useTranslation } from "react-i18next";
import RuleEventName from "./RuleEventName";
import type { EventRule, EventRuleFrequency } from "../../../types";
import { frequencyOperators, operatorTypes, periodUnits } from "./RuleHelpers";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";

interface EventRuleEditProps {
  rule: EventRule;
  eventName?: string;
  setRule: (rule: EventRule) => void;
}

export default function EventRuleEdit({
  rule,
  setRule,
  eventName,
}: EventRuleEditProps) {
  const { t } = useTranslation();

  if (eventName) {
    if (rule.children?.length) {
      return (
      <>
          {t("rule_matching")}
          <Select
            value={rule.operator}
            onValueChange={(operator: "and" | "or") =>
              setRule({ ...rule, operator })
            }
          >
            <SelectTrigger className="h-8 w-auto inline-flex text-sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {operatorTypes.wrapper.map((opt) => (
                <SelectItem key={opt.key} value={opt.key}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {t("rule_of_the_following")}
        </>
      );
    }
    return <></>;
  }

  const frequency = rule.frequency ?? {
    period: {
      type: "rolling",
      unit: "day",
      value: 30,
    },
    operator: ">=",
    count: 1,
  };

  // If missing frequency, set default values
  if (!rule.frequency) {
    setRule({
      ...rule,
      frequency,
    });
  }

  const operatorOption = frequencyOperators.find(
    (opt) => opt.key === frequency.operator
  );
  const rollingPeriod =
    frequency.period.type === "rolling" ? frequency.period : null;
  const unitOption = rollingPeriod
    ? periodUnits.find((opt) => opt.key === rollingPeriod.unit)
    : undefined;

  const updateRollingPeriod = (periodUpdate: Partial<typeof frequency.period>) => {
    const period = frequency.period;
    if (period.type !== "rolling") return;
    setRule({
      ...rule,
      frequency: {
        ...frequency,
        period: {
          ...period,
          ...periodUpdate,
        },
      },
    });
  };

  return (
    <>
      {t("rule_did")}
      <span className="inline-flex mx-1">
        <RuleEventName rule={rule} setRule={setRule} />
      </span>
      <div className="inline-flex items-center gap-1 mx-1">
        <Select
          value={frequency.operator}
          onValueChange={(operator: EventRuleFrequency["operator"]) =>
            setRule({
              ...rule,
              frequency: {
                ...(rule.frequency ?? frequency),
                operator,
              },
            })
          }
        >
          <SelectTrigger className="h-8 w-auto text-sm">
            <SelectValue placeholder={operatorOption?.label}>
              {operatorOption?.label}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {frequencyOperators.map((opt) => (
              <SelectItem key={opt.key} value={opt.key}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          type="number"
          min={1}
          className="h-8 w-16 text-sm"
          placeholder="Count"
          value={frequency.count?.toString() ?? "1"}
          onChange={(e) => {
            const count = e.target.value
              ? parseInt(e.target.value, 10)
              : undefined;
            setRule({
              ...rule,
              frequency: {
                ...(rule.frequency ?? frequency),
                count,
              },
            });
          }}
        />
      </div>
      {"times"}
      {frequency.period.type === "rolling" && (
        <>
          {" in last"}
          <div className="inline-flex items-center gap-1 mx-1">
            <Input
              type="number"
              min={1}
              className="h-8 w-16 text-sm"
              placeholder="Value"
              value={frequency.period.value.toString()}
              onChange={(e) => {
                updateRollingPeriod({ value: parseInt(e.target.value, 10) || 1 });
              }}
            />
            <Select
              value={frequency.period.unit}
              onValueChange={(unit: typeof frequency.period.unit) => {
                updateRollingPeriod({ unit });
              }}
            >
              <SelectTrigger className="h-8 w-auto text-sm">
                <SelectValue placeholder={unitOption?.label}>
                  {unitOption?.label}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {periodUnits.map((opt) => (
                  <SelectItem key={opt.key} value={opt.key}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </>
      )}
      {!!rule.children?.length && (
        <>
          {t("rule_matching")}
          <Select
            value={rule.operator}
            onValueChange={(operator: "and" | "or") =>
              setRule({ ...rule, operator })
            }
          >
            <SelectTrigger className="h-8 w-auto inline-flex text-sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {operatorTypes.wrapper.map((opt) => (
                <SelectItem key={opt.key} value={opt.key}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {t("rule_of_the_following")}
        </>
      )}
    </>
  );
}
