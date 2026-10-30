import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Check, Languages, Pencil } from "lucide-react"
import { UserContext } from "../../contexts"
import { cn } from "../../utils"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { useUpdateUser } from "./queries"

// eslint-disable-next-line @typescript-eslint/no-namespace
declare namespace Intl {
    class DisplayNames {
        constructor(locales: string[], options: { type: string })
        of(code: string): string | undefined
    }
}

/** The user's locale in the user page header, editable in a popover. */
export function UserLocaleEdit() {
    const { t } = useTranslation()
    const [user] = useContext(UserContext)
    const [isLocaleOpen, setIsLocaleOpen] = useState(false)
    const updateLocale = useUpdateUser(t("locale_update_error", "Failed to update locale"))
    const isSavingLocale = updateLocale.isPending

    const handleLocaleChange = (newLocale: string) => {
        updateLocale.mutate(
            { locale: newLocale },
            {
                onSuccess: () => {
                    toast.success(t("locale_updated", "Locale updated"))
                    setIsLocaleOpen(false)
                },
            },
        )
    }

    return user.locale ? (
        <>
            <Popover open={isLocaleOpen} onOpenChange={setIsLocaleOpen}>
                <PopoverTrigger asChild>
                    <button
                        type="button"
                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group"
                    >
                        <Languages className="h-3 w-3" />
                        <span>{user.locale}</span>
                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                    </button>
                </PopoverTrigger>
                <PopoverContent className="w-72 p-0" align="start">
                    <Command>
                        <CommandInput placeholder={t("search_locale", "Search locale...")} />
                        <CommandList>
                            <CommandEmpty>{t("no_locale_found", "No locale found.")}</CommandEmpty>
                            <CommandGroup>
                                {(() => {
                                    const locales = [
                                        ...new Set([
                                            ...navigator.languages,
                                            "en",
                                            "en",
                                            "en-GB",
                                            "es",
                                            "es-ES",
                                            "es-MX",
                                            "fr",
                                            "fr-FR",
                                            "de",
                                            "de-DE",
                                            "it",
                                            "it-IT",
                                            "pt",
                                            "pt-BR",
                                            "pt-PT",
                                            "nl",
                                            "nl-NL",
                                            "ja",
                                            "ja-JP",
                                            "ko",
                                            "ko-KR",
                                            "zh",
                                            "zh-CN",
                                            "zh-TW",
                                            "ar",
                                            "ar-SA",
                                            "hi",
                                            "hi-IN",
                                            "ru",
                                            "ru-RU",
                                            "pl",
                                            "pl-PL",
                                            "tr",
                                            "tr-TR",
                                            "sv",
                                            "sv-SE",
                                            "da",
                                            "da-DK",
                                            "no",
                                            "nb-NO",
                                            "fi",
                                            "fi-FI",
                                            "th",
                                            "th-TH",
                                            "vi",
                                            "vi-VN",
                                            "id",
                                            "id-ID",
                                            "ms",
                                            "ms-MY",
                                            "uk",
                                            "uk-UA",
                                            "cs",
                                            "cs-CZ",
                                            "ro",
                                            "ro-RO",
                                            "hu",
                                            "hu-HU",
                                            "el",
                                            "el-GR",
                                            "he",
                                            "he-IL",
                                            "bg",
                                            "bg-BG",
                                            "hr",
                                            "hr-HR",
                                            "sk",
                                            "sk-SK",
                                            "sl",
                                            "sl-SI",
                                            "lt",
                                            "lt-LT",
                                            "lv",
                                            "lv-LV",
                                            "et",
                                            "et-EE",
                                            "ca",
                                            "ca-ES",
                                            "fil",
                                            "fil-PH",
                                            "sw",
                                            "sw-KE",
                                            "af",
                                            "af-ZA",
                                        ]),
                                    ].sort()
                                    return locales.map((loc) => {
                                        let label = loc
                                        try {
                                            const dn = new Intl.DisplayNames(["en"], {
                                                type: "language",
                                            })
                                            label = dn.of(loc) ?? loc
                                        } catch {
                                            // keep raw code
                                        }
                                        return (
                                            <CommandItem
                                                key={loc}
                                                value={`${loc} ${label}`}
                                                onSelect={() => handleLocaleChange(loc)}
                                                disabled={isSavingLocale}
                                            >
                                                <Check
                                                    className={cn(
                                                        "mr-2 h-4 w-4",
                                                        user.locale === loc
                                                            ? "opacity-100"
                                                            : "opacity-0",
                                                    )}
                                                />
                                                <span>{label}</span>
                                                {label !== loc && (
                                                    <span className="ml-1 text-muted-foreground">
                                                        {loc}
                                                    </span>
                                                )}
                                            </CommandItem>
                                        )
                                    })
                                })()}
                            </CommandGroup>
                        </CommandList>
                    </Command>
                </PopoverContent>
            </Popover>
            <span className="mx-2">·</span>
        </>
    ) : (
        <>
            <Popover open={isLocaleOpen} onOpenChange={setIsLocaleOpen}>
                <PopoverTrigger asChild>
                    <button
                        type="button"
                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors text-muted-foreground/60 hover:text-muted-foreground group"
                    >
                        <Languages className="h-3 w-3" />
                        {t("set_locale", "Set locale")}
                        <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                    </button>
                </PopoverTrigger>
                <PopoverContent className="w-72 p-0" align="start">
                    <Command>
                        <CommandInput placeholder={t("search_locale", "Search locale...")} />
                        <CommandList>
                            <CommandEmpty>{t("no_locale_found", "No locale found.")}</CommandEmpty>
                            <CommandGroup>
                                {(() => {
                                    const locales = [
                                        ...new Set([
                                            ...navigator.languages,
                                            "en",
                                            "en",
                                            "en-GB",
                                            "es",
                                            "es-ES",
                                            "es-MX",
                                            "fr",
                                            "fr-FR",
                                            "de",
                                            "de-DE",
                                            "it",
                                            "it-IT",
                                            "pt",
                                            "pt-BR",
                                            "pt-PT",
                                            "nl",
                                            "nl-NL",
                                            "ja",
                                            "ja-JP",
                                            "ko",
                                            "ko-KR",
                                            "zh",
                                            "zh-CN",
                                            "zh-TW",
                                            "ar",
                                            "ar-SA",
                                            "hi",
                                            "hi-IN",
                                            "ru",
                                            "ru-RU",
                                            "pl",
                                            "pl-PL",
                                            "tr",
                                            "tr-TR",
                                            "sv",
                                            "sv-SE",
                                            "da",
                                            "da-DK",
                                            "no",
                                            "nb-NO",
                                            "fi",
                                            "fi-FI",
                                            "th",
                                            "th-TH",
                                            "vi",
                                            "vi-VN",
                                            "id",
                                            "id-ID",
                                            "ms",
                                            "ms-MY",
                                            "uk",
                                            "uk-UA",
                                            "cs",
                                            "cs-CZ",
                                            "ro",
                                            "ro-RO",
                                            "hu",
                                            "hu-HU",
                                            "el",
                                            "el-GR",
                                            "he",
                                            "he-IL",
                                            "bg",
                                            "bg-BG",
                                            "hr",
                                            "hr-HR",
                                            "sk",
                                            "sk-SK",
                                            "sl",
                                            "sl-SI",
                                            "lt",
                                            "lt-LT",
                                            "lv",
                                            "lv-LV",
                                            "et",
                                            "et-EE",
                                            "ca",
                                            "ca-ES",
                                            "fil",
                                            "fil-PH",
                                            "sw",
                                            "sw-KE",
                                            "af",
                                            "af-ZA",
                                        ]),
                                    ].sort()
                                    return locales.map((loc) => {
                                        let label = loc
                                        try {
                                            const dn = new Intl.DisplayNames(["en"], {
                                                type: "language",
                                            })
                                            label = dn.of(loc) ?? loc
                                        } catch {
                                            // keep raw code
                                        }
                                        return (
                                            <CommandItem
                                                key={loc}
                                                value={`${loc} ${label}`}
                                                onSelect={() => handleLocaleChange(loc)}
                                                disabled={isSavingLocale}
                                            >
                                                <span>{label}</span>
                                                {label !== loc && (
                                                    <span className="ml-1 text-muted-foreground">
                                                        {loc}
                                                    </span>
                                                )}
                                            </CommandItem>
                                        )
                                    })
                                })()}
                            </CommandGroup>
                        </CommandList>
                    </Command>
                </PopoverContent>
            </Popover>
            <span className="mx-2">·</span>
        </>
    )
}
