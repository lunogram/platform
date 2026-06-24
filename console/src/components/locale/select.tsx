import { useContext, useState, useEffect, useMemo } from "react"
import { LocaleContext, ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import type { Locale } from "@/types"

import { Plus, Check, ChevronsUpDown } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/utils"

import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { useTranslation } from "react-i18next"
import { LocalePicker } from "./picker"
import { resolveLocaleName } from "./locales"

interface LocaleSelectProps {
    onChange?: (localeKey: string) => void | Promise<void>
}

export function LocaleSelect({ onChange }: LocaleSelectProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [localeSelection, setLocaleSelection] = useContext(LocaleContext)
    const [open, setOpen] = useState(false)
    const [isLoading, setIsLoading] = useState(false)
    const [isDialogOpen, setIsDialogOpen] = useState(false)

    const [locales, setLocales] = useState<Locale[]>([])
    const [searchQuery, setSearchQuery] = useState("")

    const [newLocaleKey, setNewLocaleKey] = useState<string | undefined>()
    const [isCreating, setIsCreating] = useState(false)

    useEffect(() => {
        const fetchFilteredLocales = async () => {
            setIsLoading(true)

            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/locales", {
                params: {
                    path: { projectID: project.id },
                    query: { search: searchQuery, limit: 5 },
                },
            })

            setLocales(data?.results ?? [])
            setIsLoading(false)
        }

        // Debounce search
        const timeoutId = setTimeout(fetchFilteredLocales, 300)
        return () => clearTimeout(timeoutId)
    }, [searchQuery, project?.id])

    // Keys already in the project — exclude from the "add" picker
    const existingKeys = useMemo(
        () => localeSelection.allLocales.map((l) => l.key),
        [localeSelection.allLocales],
    )

    const handleSelectChange = async (value: string) => {
        const selectedLocale = locales.find((locale) => locale.key === value)

        if (selectedLocale) {
            if (onChange) await onChange(selectedLocale.key)

            setLocaleSelection((prev) => ({
                ...prev,
                currentLocale: selectedLocale,
            }))
        }

        setOpen(false)
    }

    const openDialog = () => {
        setOpen(false)
        setIsDialogOpen(true)
    }

    const handleCreateLocale = async () => {
        if (!newLocaleKey) return
        setIsCreating(true)
        try {
            const label = resolveLocaleName(newLocaleKey)
            const { data: newLocale } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/locales",
                {
                    params: { path: { projectID: project.id } },
                    body: { key: newLocaleKey, label },
                },
            )
            if (!newLocale) return

            setLocaleSelection((prev) => ({
                ...prev,
                currentLocale: newLocale,
                allLocales: [...prev.allLocales, newLocale],
            }))

            setLocales((prev) => [newLocale, ...prev])

            if (onChange) await onChange(newLocale.key)

            setNewLocaleKey(undefined)
            setIsDialogOpen(false)
        } finally {
            setIsCreating(false)
        }
    }

    const handleDialogChange = (open: boolean) => {
        setIsDialogOpen(open)
        if (!open) {
            setNewLocaleKey(undefined)
        }
    }

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
                        {localeSelection.currentLocale ? (
                            <span className="flex items-center gap-1.5 truncate">
                                <span className="truncate">
                                    {localeSelection.currentLocale.label}
                                </span>
                                <span className="text-xs text-muted-foreground">
                                    {localeSelection.currentLocale.key}
                                </span>
                            </span>
                        ) : (
                            t("locale.select.placeholder")
                        )}
                        <ChevronsUpDown className="h-4 w-4 opacity-50" />
                    </Button>
                </PopoverTrigger>
                <PopoverContent className="w-52 p-0">
                    <Command shouldFilter={false}>
                        <CommandInput
                            placeholder={t("locale.select.search_placeholder")}
                            className="h-9"
                            value={searchQuery}
                            onValueChange={setSearchQuery}
                        />
                        <CommandList>
                            <CommandEmpty>
                                {isLoading
                                    ? t("locale.select.loading")
                                    : t("locale.select.no_locale_found")}
                            </CommandEmpty>
                            <CommandGroup>
                                {locales.map((locale) => (
                                    <CommandItem
                                        className="cursor-pointer"
                                        key={locale.key}
                                        value={locale.key}
                                        onSelect={() => handleSelectChange(locale.key)}
                                    >
                                        <div className="flex items-center gap-1.5 flex-1 min-w-0">
                                            <span className="truncate">{locale.label}</span>
                                            <span className="text-xs text-muted-foreground shrink-0">
                                                {locale.key}
                                            </span>
                                        </div>
                                        <Check
                                            className={cn(
                                                "ml-auto h-4 w-4",
                                                localeSelection.currentLocale?.key === locale.key
                                                    ? "opacity-100"
                                                    : "opacity-0",
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
                                    <span>{t("locale.select.create_new")}</span>
                                </Button>
                            </div>
                        </CommandList>
                    </Command>
                </PopoverContent>
            </Popover>

            <Dialog open={isDialogOpen} onOpenChange={handleDialogChange}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("locale.select.dialog.title")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "locale.add_description",
                                "Select a language to add to this project.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label>{t("locale.picker.label", "Language")}</Label>
                            <LocalePicker
                                value={newLocaleKey}
                                onChange={setNewLocaleKey}
                                exclude={existingKeys}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => handleDialogChange(false)}
                            disabled={isCreating}
                        >
                            {t("locale.select.dialog.cancel")}
                        </Button>
                        <Button onClick={handleCreateLocale} disabled={!newLocaleKey || isCreating}>
                            {isCreating
                                ? t("creating", "Creating...")
                                : t("locale.select.dialog.create")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
