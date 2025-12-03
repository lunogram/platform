import type { ReactNode } from 'react';
import { useCallback, useContext, useMemo } from 'react'
import type { FieldPath, FieldValues } from 'react-hook-form';
import { useController } from 'react-hook-form'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import { useResolver } from '../../hooks'
import type { ControlledInputProps, FieldBindingsProps } from '../../types'
import { MultiSelect } from '@/components/ui/multi-select'
import { snakeToTitle } from '../../utils'
import { Label } from '@/components/ui/label'

export interface TagPickerProps extends ControlledInputProps<string[]> {
    entity?: 'journeys' | 'campaigns' | 'users' | 'lists'
    placeholder?: ReactNode
}

export function TagPicker({
    entity,
    value,
    label,
    error,
    required,
    placeholder,
    onChange,
    disabled,
    hideLabel,
    subtitle,
}: TagPickerProps) {
    const [project] = useContext(ProjectContext)
    const [tags] = useResolver(useCallback(async () => {
        if (entity) {
            return await api.tags.used(project.id, entity)
        }
        return await api.tags.all(project.id)
    }, [project, entity]))

    const selectedValue = useMemo(() => value ?? [], [value])
    
    const options = useMemo(() => 
        tags?.map(tag => ({
            value: tag.name,
            label: tag.count !== undefined ? `${tag.name} (${tag.count})` : tag.name
        })) ?? [],
        [tags]
    )

    if (!tags?.length) return null

    return (
        <div className="ui-select">
            {label && (
                <Label style={hideLabel ? { display: 'none' } : { lineHeight: '20px' }}>
                    <span>
                        {label}
                        {required && <span style={{ color: 'red' }}>&nbsp;*</span>}
                    </span>
                </Label>
            )}
            {subtitle && <span className="label-subtitle">{subtitle}</span>}
            <MultiSelect
                value={selectedValue}
                options={options}
                onChange={onChange}
                disabled={disabled}
                placeholder={typeof placeholder === 'string' ? placeholder : 'Select tags...'}
            />
            {error && !hideLabel && (
                <span className="field-error">{error}</span>
            )}
        </div>
    )
}

TagPicker.Field = function TagPickerField<X extends FieldValues, P extends FieldPath<X>>({
    form,
    label,
    name,
    required,
    ...rest
}: FieldBindingsProps<TagPickerProps, string[], X, P>) {

    const { field: { ...field } } = useController({
        control: form.control,
        name,
        rules: {
            required,
        },
    })

    return (
        <TagPicker
            {...rest}
            {...field}
            required={required}
            label={label ?? snakeToTitle(name)}
        />
    )
}
