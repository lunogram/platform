import { Controller, type FieldValues, type UseFormReturn } from "react-hook-form";
import { useContext, useState, useCallback, useEffect, type ComponentType } from "react";
import { CampaignContext, ProjectContext } from "@/contexts";
import type { Campaign, ChannelType, Provider, Template, User } from "@/types";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils";
import api from "@/api";

import { Check, ChevronsUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";

import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import { EmailForm, EmailFormControl, EmailPreview } from "./mail/Setup";
import { CampaignDetailContext } from "./contexts";

interface UserSelectionProps {
  projectId: string;
  value?: User | null;
  onChange?: (user: User) => void;
}

export function UserSelection({
  projectId,
  value,
  onChange,
}: UserSelectionProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [users, setUsers] = useState<User[]>([]);

  const fetchUsers = useCallback(async () => {
    const users = await api.users.search(projectId, {
      q: search,
      limit: 50,
    });

    setUsers(users.results);
  }, [projectId, search]);

  useEffect(() => {
    const handler = setTimeout(() => {
      fetchUsers();
    }, 200); // 200ms debounce

    return () => clearTimeout(handler);
  }, [search, fetchUsers]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-[200px] justify-between"
        >
          {value ? value.email : "Select user..."}
          <ChevronsUpDown className="opacity-50" />
        </Button>
      </PopoverTrigger>

      <PopoverContent className="w-[200px] p-0">
        <Command>
          <CommandInput
            placeholder="Search user..."
            className="h-9"
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            <CommandEmpty>No user found.</CommandEmpty>
            <CommandGroup>
              {users.map((user) => (
                <CommandItem
                  key={user.id}
                  value={user.email}
                  onSelect={() => {
                    onChange?.(user);
                    setOpen(false);
                  }}
                >
                  {user.email}
                  <Check
                    className={cn(
                      "ml-auto",
                      value?.id === user.id ? "opacity-100" : "opacity-0"
                    )}
                  />
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

interface ChannelConfig<T extends FieldValues> {
  form: (campaign: Campaign, template: Template) => UseFormReturn<T>;
  FormControl: ComponentType<{ campaign: Campaign; form: UseFormReturn<T> }>;
  Preview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T> }>;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const channels: Record<ChannelType, ChannelConfig<any>> = {
  email: {
    form: EmailForm,
    FormControl: EmailFormControl,
    Preview: EmailPreview,
  },
}

export default function CampaignSetup() {
  const { t } = useTranslation();
  const { onNext } = useContext(CampaignDetailContext);
  const [project] = useContext(ProjectContext);
  const [campaign, setCampaign] = useContext(CampaignContext);

  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [providers, setProviders] = useState<Provider[]>([]);

  useEffect(() => {
    const fetchProviders = async () => {
      const result = await api.providers.all(project.id);
      setProviders(result);
    };

    fetchProviders();
  }, [project?.id]);

  const handleProviderChange = (providerId: string) => {
    const provider = providers.find((p) => p.id === providerId) || undefined;
    setCampaign({
      ...campaign,
      provider_id: providerId,
      provider: provider,
    });
  }


  const config = channels[campaign.channel];
  const form = config.form(campaign, campaign.templates[0]);
  const FormControlComponent = config.FormControl;
  const PreviewComponent = config.Preview;

  onNext(async () => {
    const isValid = await form.trigger();
    if (!isValid) {
      return false;
    }

    return true;
  });

  return (
    <div className="flex flex-1 bg-muted/20 overflow-hidden flex-row bg-muted/20">
      <div className="h-full overflow-y-auto w-2/5 bg-background p-8 border-r">
        <div className="mb-4">
          <h1 className="text-2xl font-semibold">{t('campaign.setup.title')}</h1>
          <p>{t('campaign.setup.description')}</p>
        </div>

        <FieldGroup>
          <Controller
            name="name"
            control={form.control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid} className="gap-2">
                <FieldLabel htmlFor="form-rhf-demo-name">{t('campaign.setup.form.name.label')}</FieldLabel>
                <Input
                  {...field}
                  id="form-rhf-demo-name"
                  aria-invalid={fieldState.invalid}
                  placeholder=""
                  autoComplete="off"
                />
                <FieldDescription>
                  {t('campaign.setup.form.name.description')}
                </FieldDescription>
                {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
              </Field>
            )}
          />
          <Controller
            name="provider_id"
            control={form.control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid} className="gap-2">
                <FieldLabel htmlFor="form-rhf-demo-provider">{t('campaign.setup.form.provider.label')}</FieldLabel>
                <Select value={field.value} onValueChange={(value) => {
                  field.onChange(value);
                  handleProviderChange(value);
                }}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select provider" />
                  </SelectTrigger>
                  <SelectContent>
                    {providers.map((provider) => (
                      <SelectItem key={provider.id} value={provider.id}>
                        {provider.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FieldDescription>
                  {t('campaign.setup.form.provider.description')}
                </FieldDescription>
                {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
              </Field>
            )}
          />
        </FieldGroup>

        <FormControlComponent campaign={campaign} form={form} />
      </div>

      <div className="w-3/5 bg-background p-8 flex flex-col">
        <div className="flex items-center justify-between mb-6">
          <div>
            <UserSelection
              projectId={project?.id}
              value={selectedUser}
              onChange={setSelectedUser}
            />
          </div>

          <a href={`/projects/${project?.id}/campaigns`}>
            <Button variant="outline">Exit</Button>
          </a>
        </div>
        <PreviewComponent campaign={campaign} form={form} />
      </div>
    </div>
  );
}

