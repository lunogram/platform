import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"
import type { User } from "@/types"
import {
    buildPreviewProps,
    buildSchemaPaths,
    findExtraProps,
    mergeUserIntoProps,
} from "../variableScope"

export interface UsePreviewPropsResult {
    previewProps: Record<string, unknown>
    previewPropsRef: React.RefObject<Record<string, unknown>>
    propsJsonString: string
    propsValidationError: string | null
    selectedUser: User | null
    extraProps: string[]
    handlePropsJsonChange: (value: string) => void
    handlePropsReset: () => void
    handleUserSelect: (user: User) => void
    /** Ref to the stringified defaults — used by PropsEditorPanel to detect customisation */
    prevDefaultsRef: React.RefObject<string>
}

/**
 * Manages preview props state: default generation from variable groups,
 * JSON editing/validation, user selection merging, and extra-prop detection.
 */
export function usePreviewProps(variableGroups: VariableGroup[]): UsePreviewPropsResult {
    const defaultPreviewProps = useMemo(() => buildPreviewProps(variableGroups), [variableGroups])

    const [propsJsonString, setPropsJsonString] = useState<string>(() =>
        JSON.stringify(defaultPreviewProps, null, 2),
    )
    const [propsValidationError, setPropsValidationError] = useState<string | null>(null)
    const [previewProps, setPreviewProps] = useState<Record<string, unknown>>(defaultPreviewProps)
    const [selectedUser, setSelectedUser] = useState<User | null>(null)

    // Keep a ref to previewProps so BuilderCompileCheck can read without re-renders
    const previewPropsRef = useRef(previewProps)
    useEffect(() => {
        previewPropsRef.current = previewProps
    }, [previewProps])

    // When variable groups change, regenerate defaults — but only if user hasn't customised
    const prevDefaultsRef = useRef<string>(JSON.stringify(defaultPreviewProps, null, 2))
    useEffect(() => {
        const newDefaults = JSON.stringify(defaultPreviewProps, null, 2)
        if (prevDefaultsRef.current !== newDefaults) {
            if (propsJsonString === prevDefaultsRef.current) {
                setPropsJsonString(newDefaults)
                setPreviewProps(defaultPreviewProps)
                setPropsValidationError(null)
            }
            prevDefaultsRef.current = newDefaults
        }
    }, [defaultPreviewProps, propsJsonString])

    const handlePropsJsonChange = useCallback((value: string) => {
        setPropsJsonString(value)
        try {
            const parsed = JSON.parse(value) as Record<string, unknown>
            setPreviewProps(parsed)
            setPropsValidationError(null)
        } catch (err) {
            setPropsValidationError(err instanceof Error ? err.message : String(err))
        }
    }, [])

    const handlePropsReset = useCallback(() => {
        const json = JSON.stringify(defaultPreviewProps, null, 2)
        setPropsJsonString(json)
        setPreviewProps(defaultPreviewProps)
        setPropsValidationError(null)
        setSelectedUser(null)
    }, [defaultPreviewProps])

    const handleUserSelect = useCallback(
        (user: User) => {
            setSelectedUser(user)
            const merged = mergeUserIntoProps(defaultPreviewProps, user)
            const json = JSON.stringify(merged, null, 2)
            setPropsJsonString(json)
            setPreviewProps(merged)
            setPropsValidationError(null)
        },
        [defaultPreviewProps],
    )

    // Detect extra properties not in the variable schema
    const schemaPaths = useMemo(() => buildSchemaPaths(variableGroups), [variableGroups])
    const extraProps = useMemo(
        () => (propsValidationError ? [] : findExtraProps(previewProps, schemaPaths)),
        [previewProps, schemaPaths, propsValidationError],
    )

    return {
        previewProps,
        previewPropsRef,
        propsJsonString,
        propsValidationError,
        selectedUser,
        extraProps,
        handlePropsJsonChange,
        handlePropsReset,
        handleUserSelect,
        prevDefaultsRef,
    }
}
