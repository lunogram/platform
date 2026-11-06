import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Check, Globe, Pencil } from "lucide-react"
import { UserContext } from "../../contexts"
import { cn } from "../../utils"
import { getLocalTimeInTimezone, getTimezoneOffset } from "@/lib/timezone-coordinates"
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
    type Key = "calendar" | "collation" | "currency" | "numberingSystem" | "timeZone" | "unit"
    function supportedValuesOf(input: Key): string[]
}

/** The user's local time in the user page header; its timezone is editable in a popover. */
export function UserTimezoneEdit() {
    const { t } = useTranslation()
    const [user] = useContext(UserContext)
    const [isTimezoneOpen, setIsTimezoneOpen] = useState(false)
    const updateTimezone = useUpdateUser(t("timezone_update_error", "Failed to update timezone"))
    const isSavingTimezone = updateTimezone.isPending
    const timeZones = Intl.supportedValuesOf("timeZone")

    const handleTimezoneChange = (newTimezone: string) => {
        updateTimezone.mutate(
            { timezone: newTimezone },
            {
                onSuccess: () => {
                    toast.success(t("timezone_updated", "Timezone updated"))
                    setIsTimezoneOpen(false)
                },
            },
        )
    }

    return user.timezone
        ? (() => {
              const localTime = getLocalTimeInTimezone(user.timezone)
              const utcOffset = getTimezoneOffset(user.timezone)
              return (
                  <>
                      <Popover open={isTimezoneOpen} onOpenChange={setIsTimezoneOpen}>
                          <PopoverTrigger asChild>
                              <button
                                  type="button"
                                  className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors group"
                              >
                                  <Globe className="h-3 w-3" />
                                  {localTime}
                                  <span className="text-muted-foreground/60">
                                      {utcOffset && `(${utcOffset})`}
                                  </span>
                                  <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                              </button>
                          </PopoverTrigger>
                          <PopoverContent className="w-72 p-0" align="start">
                              <Command>
                                  <CommandInput
                                      placeholder={t("search_timezone", "Search timezone...")}
                                  />
                                  <CommandList>
                                      <CommandEmpty>
                                          {t("no_timezone_found", "No timezone found.")}
                                      </CommandEmpty>
                                      <CommandGroup>
                                          {timeZones.map((tz) => (
                                              <CommandItem
                                                  key={tz}
                                                  value={tz}
                                                  onSelect={() => handleTimezoneChange(tz)}
                                                  disabled={isSavingTimezone}
                                              >
                                                  <Check
                                                      className={cn(
                                                          "mr-2 h-4 w-4",
                                                          user.timezone === tz
                                                              ? "opacity-100"
                                                              : "opacity-0",
                                                      )}
                                                  />
                                                  {tz}
                                              </CommandItem>
                                          ))}
                                      </CommandGroup>
                                  </CommandList>
                              </Command>
                          </PopoverContent>
                      </Popover>
                      <span className="mx-2">·</span>
                  </>
              )
          })()
        : (() => (
              <>
                  <Popover open={isTimezoneOpen} onOpenChange={setIsTimezoneOpen}>
                      <PopoverTrigger asChild>
                          <button
                              type="button"
                              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 -mx-1.5 -my-0.5 hover:bg-muted transition-colors text-muted-foreground/60 hover:text-muted-foreground group"
                          >
                              <Globe className="h-3 w-3" />
                              {t("set_timezone", "Set timezone")}
                              <Pencil className="h-2.5 w-2.5 opacity-0 group-hover:opacity-60 transition-opacity" />
                          </button>
                      </PopoverTrigger>
                      <PopoverContent className="w-72 p-0" align="start">
                          <Command>
                              <CommandInput
                                  placeholder={t("search_timezone", "Search timezone...")}
                              />
                              <CommandList>
                                  <CommandEmpty>
                                      {t("no_timezone_found", "No timezone found.")}
                                  </CommandEmpty>
                                  <CommandGroup>
                                      {timeZones.map((tz) => (
                                          <CommandItem
                                              key={tz}
                                              value={tz}
                                              onSelect={() => handleTimezoneChange(tz)}
                                              disabled={isSavingTimezone}
                                          >
                                              {tz}
                                          </CommandItem>
                                      ))}
                                  </CommandGroup>
                              </CommandList>
                          </Command>
                      </PopoverContent>
                  </Popover>
                  <span className="mx-2">·</span>
              </>
          ))()
}
