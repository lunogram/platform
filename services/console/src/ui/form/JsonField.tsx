import TextInput, { type TextInputProps } from './TextInput'
import type { FieldProps } from '../../types'
import type { FieldPath, FieldValues } from 'react-hook-form'
import { useController } from 'react-hook-form'
import { useState } from 'react'

export default function JsonField<X extends FieldValues, P extends FieldPath<X>>({
    form,
    name,
    required,
    ...rest
}: Omit<TextInputProps<P>, 'onChange' | 'onBlur' | 'value'> & FieldProps<X, P>) {
    const { field: { ref, value, ...field }, fieldState } = useController({
        control: form.control,
        name,
        rules: {
            required,
        },
    })
    const [jsonValue, setJsonValue] = useState(JSON.stringify(value))

    return (
        <TextInput
            {...rest}
            {...field}
            value={jsonValue}
            inputRef={ref}
            onChange={async (value) => {
                setJsonValue(value)
                try {
                    field.onChange(JSON.parse(value))
                } catch { }
            }}
            required={required}
            error={fieldState.error?.message}
        />
    )
}
