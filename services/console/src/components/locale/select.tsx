import { useContext, useState, useEffect } from 'react';
import { LocaleContext, ProjectContext } from '@/contexts';
import api from '@/api';
import type { Locale } from '@/types';

import { Plus, Check, ChevronsUpDown } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/utils';

import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"

import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover"

import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { useTranslation } from 'react-i18next';

interface LocaleSelectProps {
    onChange?: (localeKey: string) => void | Promise<void>;
}

export function LocaleSelect({ onChange }: LocaleSelectProps) {
    const { t } = useTranslation();
    const [project] = useContext(ProjectContext);
    const [localeSelection, setLocaleSelection] = useContext(LocaleContext);
    const [open, setOpen] = useState(false);
    const [isLoading, setIsLoading] = useState(false);
    const [isDialogOpen, setIsDialogOpen] = useState(false);

    const [locales, setLocales] = useState<Locale[]>([]);
    const [searchQuery, setSearchQuery] = useState('');

    const [newLocaleKey, setNewLocaleKey] = useState('');
    const [newLocaleLabel, setNewLocaleLabel] = useState('');

    useEffect(() => {
        const fetchFilteredLocales = async () => {
            setIsLoading(true);

            const { results } = await api.locales.search(project.id, {
                q: searchQuery,
                limit: 5
            });

            setLocales(results);
            setIsLoading(false);
        };

        // Debounce search
        const timeoutId = setTimeout(fetchFilteredLocales, 300);
        return () => clearTimeout(timeoutId);
    }, [searchQuery, project?.id]);

    const handleSelectChange = async (value: string) => {
        const selectedLocale = locales.find(
            (locale) => locale.key === value
        );

        if (selectedLocale) {
            if (onChange) await onChange(selectedLocale.key);

            setLocaleSelection((prev) => ({
                ...prev,
                currentLocale: selectedLocale,
            }));
        }

        setOpen(false);
    };

    const openDialog = () => {
        setOpen(false);
        setIsDialogOpen(true);
    }

    const handleCreateLocale = async () => {
        const newLocale = await api.locales.create(project.id, {
            key: newLocaleKey,
            label: newLocaleLabel
        });

        setLocaleSelection((prev) => ({
            ...prev,
            currentLocale: newLocale,
            allLocales: [...prev.allLocales, newLocale],
        }));

        setLocales((prev) => [newLocale, ...prev]);

        setNewLocaleKey('');
        setNewLocaleLabel('');
        setIsDialogOpen(false);
    };

    const handleCancel = () => {
        setNewLocaleKey('');
        setNewLocaleLabel('');
        setIsDialogOpen(false);
    };

    const handleDialogChange = (open: boolean) => {
        setIsDialogOpen(open);
        if (!open) {
            // Reset form when dialog closes
            setNewLocaleKey('');
            setNewLocaleLabel('');
        }
    };

    return (
        <>
            <Popover open={open} onOpenChange={setOpen}>
                <PopoverTrigger asChild>
                    <Button
                        variant="outline"
                        role="combobox"
                        aria-expanded={open}
                        className="w-52 justify-between"
                    >
                        {localeSelection.currentLocale
                            ? localeSelection.currentLocale.label
                            : t('locale.select.placeholder')}
                        <ChevronsUpDown className="h-4 w-4 opacity-50" />
                    </Button>
                </PopoverTrigger>
                <PopoverContent className="w-52 p-0">
                    <Command shouldFilter={false}>
                        <CommandInput
                            placeholder={t('locale.select.search_placeholder')}
                            className="h-9"
                            value={searchQuery}
                            onValueChange={setSearchQuery}
                        />
                        <CommandList>
                            <CommandEmpty>
                                {isLoading ? t('locale.select.loading') : t('locale.select.no_locale_found')}
                            </CommandEmpty>
                            <CommandGroup>
                                {locales.map((locale) => (
                                    <CommandItem
                                        className="cursor-pointer"
                                        key={locale.key}
                                        value={locale.key}
                                        onSelect={() => handleSelectChange(locale.key)}
                                    >
                                        {locale.label}
                                        <Check
                                            className={cn(
                                                "ml-auto h-4 w-4",
                                                localeSelection.currentLocale?.key === locale.key
                                                    ? "opacity-100"
                                                    : "opacity-0"
                                            )}
                                        />
                                    </CommandItem>
                                ))}
                            </CommandGroup>
                            <div className="border-t">
                                <Button
                                    className="cursor-pointer rounded-none px-3 w-full justify-start"
                                    variant="ghost"
                                    onClick={openDialog}
                                >
                                    <Plus className="h-4 w-4" />
                                    <span>{t('locale.select.create_new')}</span>
                                </Button>
                            </div>
                        </CommandList>
                    </Command>
                </PopoverContent>
            </Popover>

            <Dialog open={isDialogOpen} onOpenChange={handleDialogChange}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('locale.select.dialog.title')}</DialogTitle>
                        <DialogDescription>
                            {t('locale.select.dialog.description')}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label htmlFor="locale-key">{t('locale.select.dialog.locale_key_label')}</Label>
                            <Input
                                id="locale-key"
                                placeholder={t('locale.select.dialog.locale_key_placeholder')}
                                value={newLocaleKey}
                                onChange={(e) => setNewLocaleKey(e.target.value)}
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="locale-label">{t('locale.select.dialog.locale_label_label')}</Label>
                            <Input
                                id="locale-label"
                                placeholder={t('locale.select.dialog.locale_label_placeholder')}
                                value={newLocaleLabel}
                                onChange={(e) => setNewLocaleLabel(e.target.value)}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={handleCancel}>
                            {t('locale.select.dialog.cancel')}
                        </Button>
                        <Button
                            onClick={handleCreateLocale}
                            disabled={!newLocaleKey || !newLocaleLabel}
                        >
                            {t('locale.select.dialog.create')}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    );
}