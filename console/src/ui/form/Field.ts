import type { Control, FieldPath, FieldValues, UseFormReturn } from 'react-hook-form'
import type { CommonInputProps } from '../../types'

export interface FieldProps<X extends FieldValues, P extends FieldPath<X>> extends CommonInputProps {
    form?: UseFormReturn<X>
    name: P
}

export interface FieldOption {
    key: string | number
    label: string
}

export interface SelectionProps<X extends FieldValues> {
    name: FieldPath<X>
    control: Control<X, any> | undefined
}
