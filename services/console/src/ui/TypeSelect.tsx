import type { ReactNode } from "react";
import type { FieldPath, FieldValues } from "react-hook-form";
import { useController } from "react-hook-form";
import {
  defaultGetOptionDisplay,
  defaultGetValueKey,
  defaultToValue,
} from "./utils";
import type {
  ControlledInputProps,
  FieldBindingsProps,
  OptionsProps,
} from "../types";
import { cn } from "@/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import "./form/Select.css";

export interface TypeSelectProps<T, O = T>
  extends ControlledInputProps<T>,
    OptionsProps<O, T> {
  className?: string;
  buttonClassName?: string;
  getSelectedOptionDisplay?: (option: O) => ReactNode;
  onBlur?: () => void;
  optionsFooter?: ReactNode;
  size?: "small" | "regular";
  variant?: "plain" | "minimal";
  prefix?: ReactNode;
}

export function TypeSelect<T, U = T>({
  buttonClassName,
  className,
  disabled,
  error,
  getOptionDisplay = defaultGetOptionDisplay,
  getSelectedOptionDisplay = getOptionDisplay,
  hideLabel,
  label,
  options,
  optionsFooter,
  onBlur,
  onChange,
  required,
  size,
  subtitle,
  toValue = defaultToValue,
  getValueKey = defaultGetValueKey,
  value,
  variant = "plain",
  prefix,
}: TypeSelectProps<T, U>) {
  const selectedOption = options.find((o) =>
    Object.is(getValueKey(toValue(o)), getValueKey(value))
  );

  // Convert value to string for shadcn Select
  const stringValue = value != null ? String(getValueKey(value)) : undefined;

  // Handle value change by finding the original option
  const handleValueChange = (stringVal: string) => {
    const option = options.find(
      (o) => String(getValueKey(toValue(o))) === stringVal
    );
    if (option) {
      onChange(toValue(option));
    }
  };

  return (
    <div
      className={cn(
        "ui-select",
        variant,
        prefix && "ui-select-prefix-wrapper",
        className
      )}
    >
      {label && (
        <label style={hideLabel ? { display: "none" } : undefined}>
          <span>
            {label}
            {required && <span style={{ color: "red" }}>&nbsp;*</span>}
          </span>
        </label>
      )}
      {subtitle && <span className="label-subtitle">{subtitle}</span>}
      {prefix && <span className="ui-select-prefix">{prefix}</span>}
      <Select
        value={stringValue}
        onValueChange={handleValueChange}
        disabled={disabled}
      >
        <SelectTrigger
          className={cn("select-button h-5", size, buttonClassName)}
          onBlur={onBlur}
        >
          <SelectValue asChild>
            <span className="select-button-label">
              {selectedOption ? getSelectedOptionDisplay(selectedOption) : ""}
            </span>
          </SelectValue>
        </SelectTrigger>
        <SelectContent className="select-options">
          {options.map((option) => {
            const val = toValue(option);
            const key = getValueKey(val);
            return (
              <SelectItem
                key={key}
                value={String(key)}
                className="select-option"
              >
                <span>{getOptionDisplay(option)}</span>
              </SelectItem>
            );
          })}
          {optionsFooter}
        </SelectContent>
      </Select>
      {error && !hideLabel && <span className="field-error">{error}</span>}
    </div>
  );
}

TypeSelect.Field = function TypeSelectField<
  T,
  O,
  X extends FieldValues,
  P extends FieldPath<X>,
>({
  form,
  name,
  required,
  ...rest
}: FieldBindingsProps<TypeSelectProps<T, O>, T, X, P>) {
  const {
    field: { ...field },
    fieldState,
  } = useController({
    control: form.control,
    name,
    rules: {
      required,
    },
  });

  return (
    <TypeSelect
      {...rest}
      {...field}
      required={required}
      error={fieldState.error?.message}
    />
  );
};
