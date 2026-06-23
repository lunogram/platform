import { useMemo, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDown, ChevronRight, SlidersHorizontal } from "lucide-react"
import { snakeToTitle } from "@/utils"
import type { GrantVerb, PermissionGrant, ProjectRole } from "@/types"
import { grantVerbs } from "@/types"
import { presetGrants, resourceGroups, rolePresets } from "@/lib/rbac-presets"

import { Checkbox } from "@/components/ui/checkbox"
import { SelectableCard } from "@/components/ui/selectable-card"

// PermissionSelection is either one of the role presets or a custom set of
// (resource, verb) grants. Picking a preset pre-checks the matrix; editing any
// cell switches the selection to custom.
export type PermissionSelection =
    | { kind: "role"; role: ProjectRole }
    | { kind: "custom"; grants: PermissionGrant[] }

const grantKey = (resource: string, verb: GrantVerb) => `${resource}:${verb}`
const allResources = resourceGroups.flatMap((g) => g.resources)

export interface PermissionSelectorProps {
    selection: PermissionSelection
    onChange: (selection: PermissionSelection) => void
    // renderResourceAside, when provided, may return a node rendered inline in a
    // resource's label cell, after its name (e.g. a per-resource create-scope
    // pill). Return null/undefined for resources with no aside.
    renderResourceAside?: (resource: string) => ReactNode
}

export default function PermissionSelector({
    selection,
    onChange,
    renderResourceAside,
}: PermissionSelectorProps) {
    const { t } = useTranslation()

    // The granted (resource, verb) cells for the current selection: the expanded
    // preset when a role is active, the explicit grants when custom.
    const granted = useMemo(() => {
        const set = new Set<string>()
        const grants = selection.kind === "custom" ? selection.grants : presetGrants(selection.role)
        for (const g of grants) set.add(grantKey(g.resource, g.verb))
        return set
    }, [selection])

    const isCustom = selection.kind === "custom"
    const activeRole = selection.kind === "role" ? selection.role : null

    // Resource groups collapse to keep the matrix compact. Start with the
    // first group open, plus any group that already has a grant so existing
    // configuration is visible.
    const [expanded, setExpanded] = useState<Set<string>>(() => {
        const init = new Set<string>([resourceGroups[0].label])
        const grants = selection.kind === "custom" ? selection.grants : presetGrants(selection.role)
        const keys = new Set(grants.map((g) => grantKey(g.resource, g.verb)))
        for (const group of resourceGroups) {
            if (group.resources.some((r) => grantVerbs.some((v) => keys.has(grantKey(r, v)))))
                init.add(group.label)
        }
        return init
    })
    const toggleGroup = (label: string) =>
        setExpanded((prev) => {
            const next = new Set(prev)
            if (next.has(label)) next.delete(label)
            else next.add(label)
            return next
        })

    // Any edit to the matrix produces a custom selection (even from a preset).
    const commit = (next: Set<string>) => {
        const grants: PermissionGrant[] = Array.from(next).map((k) => {
            const [r, v] = k.split(":")
            return { resource: r, verb: v as GrantVerb }
        })
        onChange({ kind: "custom", grants })
    }

    const toggle = (resource: string, verb: GrantVerb) => {
        const next = new Set(granted)
        const key = grantKey(resource, verb)
        if (next.has(key)) next.delete(key)
        else next.add(key)
        commit(next)
    }

    // setMany toggles a verb across every resource in a group at once.
    const setMany = (group: string[], verb: GrantVerb, value: boolean) => {
        const next = new Set(granted)
        for (const resource of group) {
            const key = grantKey(resource, verb)
            if (value) next.add(key)
            else next.delete(key)
        }
        commit(next)
    }

    // setAll grants (or clears) every resource and verb — "access all resources".
    const setAll = (value: boolean) => {
        if (!value) return commit(new Set())
        const next = new Set<string>()
        for (const resource of allResources)
            for (const verb of grantVerbs) next.add(grantKey(resource, verb))
        commit(next)
    }

    const allChecked = allResources.length * grantVerbs.length === granted.size

    return (
        <div className="grid gap-3">
            {/* Preset cards: the role bundles plus a Custom indicator. */}
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                {rolePresets.map((preset) => (
                    <SelectableCard
                        key={preset.role}
                        title={preset.label}
                        summary={preset.summary}
                        active={activeRole === preset.role}
                        onClick={() => onChange({ kind: "role", role: preset.role })}
                    />
                ))}
                <SelectableCard
                    title={t("custom", "Custom")}
                    summary={t("custom_permissions_summary", "Hand-pick resources and verbs.")}
                    icon={<SlidersHorizontal className="h-4 w-4" />}
                    active={isCustom}
                    onClick={() => commit(new Set(granted))}
                />
            </div>

            {/* The matrix is always shown and editable. A preset pre-checks it;
                editing a cell switches to Custom. */}
            <div className="overflow-hidden rounded-lg border">
                <div className="flex items-center justify-end gap-3 border-b bg-surface-soft px-3 py-2">
                    <button
                        type="button"
                        onClick={() => setAll(!allChecked)}
                        className="shrink-0 text-xs font-medium text-ink-soft transition-colors hover:text-primary"
                    >
                        {allChecked ? t("clear_all", "Clear all") : t("select_all", "Select all")}
                    </button>
                </div>
                <table className="w-full text-sm">
                    <thead className="bg-card">
                        <tr className="border-b">
                            <th className="px-3 py-2 text-left font-medium text-ink-soft">
                                {t("resource", "Resource")}
                            </th>
                            {grantVerbs.map((verb) => (
                                <th
                                    key={verb}
                                    className="px-2 py-2 text-center font-medium capitalize text-ink-soft"
                                >
                                    {verb}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {resourceGroups.map((group) => (
                            <GroupRows
                                key={group.label}
                                label={group.label}
                                resources={group.resources}
                                granted={granted}
                                expanded={expanded.has(group.label)}
                                onToggleExpand={() => toggleGroup(group.label)}
                                onToggle={toggle}
                                onSetGroup={setMany}
                                renderResourceAside={renderResourceAside}
                            />
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    )
}

function GroupRows({
    label,
    resources,
    granted,
    expanded,
    onToggleExpand,
    onToggle,
    onSetGroup,
    renderResourceAside,
}: {
    label: string
    resources: string[]
    granted: Set<string>
    expanded: boolean
    onToggleExpand: () => void
    onToggle: (resource: string, verb: GrantVerb) => void
    onSetGroup: (resources: string[], verb: GrantVerb, value: boolean) => void
    renderResourceAside?: (resource: string) => ReactNode
}) {
    const grantedCount = resources.reduce(
        (n, r) => n + grantVerbs.filter((v) => granted.has(grantKey(r, v))).length,
        0,
    )
    const Chevron = expanded ? ChevronDown : ChevronRight
    return (
        <>
            {/* Group header: a disclosure for the rows, plus a tri-state checkbox
                per verb that toggles that verb across the whole group. */}
            <tr className="bg-surface-soft">
                <td className="py-2 pl-2 pr-3">
                    <button
                        type="button"
                        onClick={onToggleExpand}
                        className="flex items-center gap-1 text-xs font-medium uppercase tracking-wide text-ink-soft"
                    >
                        <Chevron className="h-3.5 w-3.5" />
                        {label}
                        {!expanded && grantedCount > 0 && (
                            <span className="font-mono lowercase tracking-normal text-ink-soft">
                                · {grantedCount}
                            </span>
                        )}
                    </button>
                </td>
                {grantVerbs.map((verb) => {
                    const count = resources.filter((r) => granted.has(grantKey(r, verb))).length
                    const state =
                        count === 0 ? false : count === resources.length ? true : "indeterminate"
                    return (
                        <td key={verb} className="px-2 py-2 text-center">
                            <Checkbox
                                checked={state}
                                onCheckedChange={() => onSetGroup(resources, verb, state !== true)}
                                aria-label={`${label} — all ${verb}`}
                            />
                        </td>
                    )
                })}
            </tr>
            {expanded &&
                resources.map((resource) => (
                    <tr key={resource} className="border-b last:border-0">
                        <td className="py-2 pl-8 pr-3">
                            <div className="flex items-center gap-2">
                                <span>{snakeToTitle(resource)}</span>
                                {renderResourceAside?.(resource)}
                            </div>
                        </td>
                        {grantVerbs.map((verb) => (
                            <td key={verb} className="px-2 py-2 text-center">
                                <Checkbox
                                    checked={granted.has(grantKey(resource, verb))}
                                    onCheckedChange={() => onToggle(resource, verb)}
                                    aria-label={`${resource} ${verb}`}
                                />
                            </td>
                        ))}
                    </tr>
                ))}
        </>
    )
}
