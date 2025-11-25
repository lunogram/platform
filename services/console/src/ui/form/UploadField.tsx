import React, { useId, useState } from 'react'
import type { FieldPath, FieldValues } from 'react-hook-form'
import { useController } from 'react-hook-form'
import type { FieldProps } from './Field'
import './UploadField.css'

interface UploadFieldProps<X extends FieldValues, P extends FieldPath<X>> extends FieldProps<X, P> {
    value?: FileList | null
    onChange?: (value: FileList) => void
    isUploading?: boolean
    accept?: string
}

export default function UploadField<X extends FieldValues, P extends FieldPath<X>>(props: UploadFieldProps<X, P>) {
    const id = useId()
    const {
        disabled,
        form,
        label,
        name,
        required,
        isUploading = false,
        accept = 'text/csv',
    } = props
    // Always call useController to satisfy rules of hooks
    const controllerResult = useController({ name, control: form?.control, disabled: !form })
    const value = form ? controllerResult.field.value : props.value
    const onChange = form ? controllerResult.field.onChange : props.onChange

    const [isHighlighted, setIsHighlighted] = useState(false)

    const dragEnter = (event: React.DragEvent<HTMLLabelElement>) => {
        setIsHighlighted(true)
        event.preventDefault()
        event.stopPropagation()
    }

    const dragExit = (event: React.DragEvent<HTMLLabelElement>) => {
        setIsHighlighted(false)
        event.preventDefault()
        event.stopPropagation()
    }

    const drop = (event: React.DragEvent<HTMLLabelElement>) => {
        dragExit(event)
        onChange?.(event.dataTransfer.files)
    }

    return (
        <label className={`legacy ui-upload-field ${isHighlighted ? 'highlighted' : ''}`}
            onDragEnter={dragEnter}
            onDragOver={dragEnter}
            onDragLeave={dragExit}
            onDrop={drop}>
            <span>
                {label ?? name}
                {required && <span style={{ color: 'red' }}>*</span>}
            </span>

            <p>{value?.[0]
                ? isUploading
                    ? `Uploading ${value?.[0].name} ...`
                    : value?.[0].name
                : 'Click to select file or drop one in.'}</p>
            <input
                type="file"
                id={id}
                className="legacy"
                name={name}
                accept={accept}
                required={required}
                disabled={disabled ?? isUploading}
                onChange={(event) => event.target.files && onChange?.(event.target.files)} />
        </label>
    )
}
