import { useState, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Check, ChevronsUpDown, Globe } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from '@/components/ui/command'
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/utils'

import { LOCALES, searchLocales, type LocaleEntry } from './locales'

// ---------------------------------------------------------------------------
// LocalePicker – a searchable combobox backed by the standardised locale list.
//
// Use-cases:
//   1. ProjectForm   – pick the project's default locale
//   2. Locales page  – add a new project locale
//   3. Anywhere a single locale key needs to be chosen
// ---------------------------------------------------------------------------

interface LocalePickerProps {
    /** Currently selected locale key (e.g. "en-US") */
    value?: string
    /** Called when the user selects a locale */
    onChange: (key: string) => void
    /** Optional list of keys to exclude (already added) */
    exclude?: string[]
    /** Placeholder text when nothing is selected */
    placeholder?: string
    /** Additional CSS classes for the trigger button */
    className?: string
    /** Whether the picker is disabled */
    disabled?: boolean
}

export function LocalePicker({
    value,
    onChange,
    exclude,
    placeholder,
    className,
    disabled,
}: LocalePickerProps) {
    const { t } = useTranslation()
    const [open, setOpen] = useState(false)
    const [query, setQuery] = useState('')

    const excludeSet = useMemo(
        () => new Set(exclude ?? []),
        [exclude],
    )

    const filteredLocales = useMemo(() => {
        const results = searchLocales(query)
        if (excludeSet.size === 0) return results
        return results.filter(l => !excludeSet.has(l.key))
    }, [query, excludeSet])

    const selectedEntry = useMemo(
        () => LOCALES.find(l => l.key === value),
        [value],
    )

    const handleSelect = useCallback(
        (key: string) => {
            onChange(key)
            setOpen(false)
            setQuery('')
        },
        [onChange],
    )

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    disabled={disabled}
                    className={cn('w-full justify-between', className)}
                >
                    {selectedEntry ? (
                        <span className="flex items-center gap-2 truncate">
                            <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                            <span className="truncate">{selectedEntry.label}</span>
                            <span className="text-xs text-muted-foreground">{selectedEntry.key}</span>
                        </span>
                    ) : (
                        <span className="text-muted-foreground">
                            {placeholder ?? t('locale.picker.placeholder', 'Select a locale...')}
                        </span>
                    )}
                    <ChevronsUpDown className="ml-auto h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[340px] p-0" align="start">
                <Command shouldFilter={false}>
                    <CommandInput
                        placeholder={t('locale.picker.search', 'Search languages...')}
                        value={query}
                        onValueChange={setQuery}
                        className="h-9"
                    />
                    <CommandList>
                        <CommandEmpty>
                            {t('locale.picker.empty', 'No matching locale found.')}
                        </CommandEmpty>
                        <CommandGroup>
                            {filteredLocales.slice(0, 50).map((locale) => (
                                <CommandItem
                                    key={locale.key}
                                    value={locale.key}
                                    onSelect={() => handleSelect(locale.key)}
                                    className="cursor-pointer"
                                >
                                    <div className="flex items-center gap-2 flex-1 min-w-0">
                                        <span className="truncate">{locale.label}</span>
                                        <span className="text-xs text-muted-foreground shrink-0">
                                            {locale.key}
                                        </span>
                                    </div>
                                    <Check
                                        className={cn(
                                            'ml-auto h-4 w-4 shrink-0',
                                            value === locale.key ? 'opacity-100' : 'opacity-0',
                                        )}
                                    />
                                </CommandItem>
                            ))}
                        </CommandGroup>
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}
