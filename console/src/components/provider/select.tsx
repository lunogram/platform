import { useState, useEffect, useContext } from "react";
import { ProjectContext } from "@/contexts";
import api from "@/api";
import type { Provider, ProviderGroup } from "@/types";
import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface ProviderSelectProps {
  value?: string;
  onChange?: (value: string) => void;
  channel: ProviderGroup;
}

export function ProviderSelect({
  value,
  onChange,
  channel,
}: ProviderSelectProps) {
  const { t } = useTranslation();
  const [project] = useContext(ProjectContext);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    const fetchProviders = async () => {
      setIsLoading(true);
      try {
        const result = await api.providers.all(project.id);
        const filteredProviders = result.filter((provider) => provider.channel === channel);
        setProviders(filteredProviders);

        if (filteredProviders.length > 0 && !value) {
          const defaultProvider =
            filteredProviders.find((provider) => provider.is_default) ?? filteredProviders[0];
          onChange?.(defaultProvider.id);
        }
      } finally {
        setIsLoading(false);
      }
    };

    fetchProviders();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project.id, channel]);

  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="w-full">
        <SelectValue placeholder={t("provider.select.placeholder")} />
      </SelectTrigger>
      <SelectContent>
        {isLoading ? (
          <div className="py-2 px-2 text-sm text-muted-foreground">
            {t("provider.select.loading")}
          </div>
        ) : providers.length === 0 ? (
          <div className="py-2 px-2 text-sm text-muted-foreground">
            {t("provider.select.no_provider_found")}
          </div>
        ) : (
          providers.map((provider) => (
            <SelectItem key={provider.id} value={provider.id}>
              {provider.name}
            </SelectItem>
          ))
        )}
      </SelectContent>
    </Select>
  );
}
