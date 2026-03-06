import clsx from "clsx"
import type { ControlledInputProps } from "../../types"
import type { FieldOption } from "./Field"

interface MultiOptionFieldProps extends ControlledInputProps<any[]> {
    options: FieldOption[]
    max?: number
}

export function MultiOptionField({
    disabled,
    label,
    onChange,
    options,
    subtitle,
    required,
    value,
    max,
}: MultiOptionFieldProps) {
    const atLimit = max !== undefined && value.length >= max

    return (
        <div className="options-group">
            {label && (
                <label role="none">
                    <span>
                        {label}
                        {required && <span style={{ color: "red" }}>&nbsp;*</span>}
                    </span>
                </label>
            )}
            {subtitle && <span className="label-subtitle">{subtitle}</span>}
            <div className="options">
                {options.map(({ key, label }) => {
                    const selected = !!value?.includes(key)
                    const isDisabled = disabled ?? (atLimit && !selected)

                    return (
                        <label key={key} className={clsx("legacy option", selected && "selected")}>
                            <input
                                className="legacy"
                                type="checkbox"
                                checked={Boolean(value?.includes(key))}
                                onChange={(e) =>
                                    onChange(
                                        e.target.checked
                                            ? value?.includes(key)
                                                ? value
                                                : [...(value ?? []), key]
                                            : (value?.filter((v) => v !== key) ?? []),
                                    )
                                }
                                style={{ display: "none" }}
                                disabled={isDisabled}
                            />
                            {label}
                        </label>
                    )
                })}
            </div>
        </div>
    )
}
