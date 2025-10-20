import { useContext, useEffect, useMemo } from 'react'
import { highlightSearch, usePopperSelectDropdown } from '../../../ui/utils'
import type { RuleEditProps } from './RuleHelpers';
import { operatorTypes, VariablesContext, ruleTypes } from './RuleHelpers'
import { ButtonGroup } from '../../../ui'
import { SingleSelect } from '../../../ui/form/SingleSelect'
import { Combobox } from '@headlessui/react'
import { ChevronUpDownIcon } from '../../../ui/icons'
import clsx from 'clsx'
import TextInput from '../../../ui/form/TextInput'
import type { RulePath } from '../../../types';
import { defaultOperator, typeOperators } from '../../../types'

export default function FilterRuleEdit({
    rule,
    setRule,
    group,
    eventName = '',
    controls,
}: Omit<RuleEditProps, 'root' | 'headerPrefix' | 'depth'>) {
    const {
        setReferenceElement,
        setPopperElement,
        attributes,
        styles,
    } = usePopperSelectDropdown()
    const { suggestions } = useContext(VariablesContext)
    const { path } = rule
    const hasValue = rule?.operator && !['is set', 'is not set', 'empty'].includes(rule?.operator)
    const pathSuggestions = useMemo<RulePath[]>(() => {
        let paths = group === 'event'
            ? eventName
                ? suggestions.eventPaths[eventName] ?? []
                : []
            : suggestions.userPaths

        if (path) {
            let search = path.toLowerCase()
            if (search.startsWith('.')) search = '$' + search
            if (!search.startsWith('$.')) search = '$.' + search
            paths = paths.filter(p => p.path.toLowerCase().startsWith(search))
        }

        return paths
    }, [suggestions, group, eventName, path])

    // NOTE: the value field could be empty when switching between operators
    useEffect(() => {
        if (!hasValue) return

        if (!typeOperators(rule.type).includes(rule.operator)) {
            setRule({ ...rule, operator: defaultOperator(rule.type) })
        }

        const isBool = rule.type === 'boolean'
        const isValidBool = rule.value === 'true' || rule.value === 'false'

        if (isBool && !isValidBool) {

            setRule({ ...rule, value: 'true' })
        }
    }, [hasValue, rule, setRule])

    return (
        <div className="rule">
            <ButtonGroup className="ui-select">
                <SingleSelect
                    value={rule.type}
                    onChange={type => setRule({ ...rule, type })}
                    options={ruleTypes}
                    required
                    hideLabel
                    size="small"
                    toValue={x => x.key as typeof rule.type}
                />
                <Combobox onChange={({ data_type: type, path }: RulePath) => setRule({ ...rule, type, path })}>
                    <span className="ui-text-input">
                        <Combobox.Input
                            value={rule.path}
                            onChange={e => setRule({ ...rule, path: e.target.value })}
                            required
                            ref={setReferenceElement}
                            className="small"
                        />
                    </span>
                    <Combobox.Button className="ui-button small secondary">
                        <ChevronUpDownIcon />
                    </Combobox.Button>
                    <Combobox.Options
                        className="select-options nowheel"
                        ref={setPopperElement}
                        style={styles.popper}
                        {...attributes.popper}
                    >
                        {
                            pathSuggestions.map(s => (
                                <Combobox.Option
                                    key={s.path}
                                    value={s}
                                    className={({ active, selected }) => clsx('select-option', active && 'active', selected && 'selected')}
                                >
                                    <span
                                        dangerouslySetInnerHTML={{
                                            __html: highlightSearch(s.path, rule.path),
                                        }}
                                    />
                                </Combobox.Option>
                            ))
                        }
                    </Combobox.Options>
                </Combobox>
                <SingleSelect
                    value={rule.operator}
                    onChange={operator => setRule({ ...rule, operator })}
                    options={operatorTypes[rule.type] ?? []}
                    required
                    hideLabel
                    size="small"
                    toValue={x => x.key}
                />
                {hasValue && rule.type === 'boolean'
                    ? (<SingleSelect
                        value={rule.value === 'true' ? 'true' : rule.value === 'false' ? 'false' : undefined}
                        onChange={value => setRule({ ...rule, value })}
                        options={[
                            { key: 'true', label: 'True' },
                            { key: 'false', label: 'False' },
                        ]}
                        required
                        hideLabel
                        size="small"
                        toValue={x => x.key}
                    />)
                    : (<TextInput
                        size="small"
                        type="text"
                        name="value"
                        placeholder="Value"
                        hideLabel={true}
                        value={rule?.value?.toString()}
                        onChange={value => setRule({ ...rule, value })}
                    />)}
                {controls}
            </ButtonGroup>
        </div>
    )
}
