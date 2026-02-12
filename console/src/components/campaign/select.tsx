import { useState, useEffect, useCallback } from 'react';
import api from '@/api';
import type { Campaign } from '@/types';
import type { UUID } from '@/types/common';

import { Check, ChevronsUpDown } from 'lucide-react';
import { Button } from '@/components/ui/button';
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

import { useTranslation } from 'react-i18next';
import { NIL } from 'uuid';
import { ChannelIcon } from '@/views/campaign/ChannelTag';

interface CampaignSelectProps {
    projectId: UUID;
    value: UUID;
    onChange: (campaignId: UUID | null) => void;
    required?: boolean;
}

export function CampaignSelect({ projectId, value, onChange, required }: CampaignSelectProps) {
    const { t } = useTranslation();
    const [open, setOpen] = useState(false);
    const [isLoading, setIsLoading] = useState(false);

    const [campaigns, setCampaigns] = useState<Campaign[]>([]);
    const [selectedCampaign, setSelectedCampaign] = useState<Campaign | null>(null);
    const [searchQuery, setSearchQuery] = useState('');

    // Fetch the selected campaign on mount if we have a value
    useEffect(() => {
        const fetchSelectedCampaign = async () => {
            if (value && value !== NIL) {
                try {
                    const campaign = await api.campaigns.get(projectId, value);
                    setSelectedCampaign(campaign);
                } catch {
                    setSelectedCampaign(null);
                }
            } else {
                setSelectedCampaign(null);
            }
        };

        fetchSelectedCampaign();
    }, [projectId, value]);

    // Fetch campaigns when search query changes
    useEffect(() => {
        const fetchCampaigns = async () => {
            setIsLoading(true);

            const { results } = await api.campaigns.search(projectId, {
                q: searchQuery,
                limit: 50,
                filter: { type: 'trigger' }
            });

            setCampaigns(results);
            setIsLoading(false);
        };

        // Debounce search
        const timeoutId = setTimeout(fetchCampaigns, 300);
        return () => clearTimeout(timeoutId);
    }, [searchQuery, projectId]);

    const handleSelectChange = useCallback((campaignId: string) => {
        const selected = campaigns.find(
            (campaign) => campaign.id === campaignId
        );

        if (selected) {
            setSelectedCampaign(selected);
            onChange(selected.id);
        }

        setOpen(false);
    }, [campaigns, onChange]);

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className="w-full justify-between"
                >
                    <span className="flex items-center gap-2 truncate">
                        {selectedCampaign && <ChannelIcon channel={selectedCampaign.channel} />}
                        <span className="truncate">
                            {selectedCampaign
                                ? selectedCampaign.name
                                : t('campaign.select.placeholder')}
                        </span>
                    </span>
                    <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent 
                className="p-0" 
                style={{ 
                    zIndex: 1001,
                    width: 'var(--radix-popover-trigger-width)'
                }}
            >
                <Command shouldFilter={false}>
                    <CommandInput
                        placeholder={t('campaign.select.search_placeholder')}
                        className="h-9"
                        value={searchQuery}
                        onValueChange={setSearchQuery}
                    />
                    <CommandList>
                        <CommandEmpty>
                            {isLoading ? t('campaign.select.loading') : t('campaign.select.no_campaign_found')}
                        </CommandEmpty>
                        <CommandGroup>
                            {campaigns.map((campaign) => (
                                <CommandItem
                                    className="cursor-pointer"
                                    key={campaign.id}
                                    value={campaign.id}
                                    onSelect={() => handleSelectChange(campaign.id)}
                                >
                                    <span className="flex items-center gap-2 truncate">
                                        <ChannelIcon channel={campaign.channel} />
                                        <span className="truncate">{campaign.name}</span>
                                    </span>
                                    <Check
                                        className={cn(
                                            "ml-auto h-4 w-4 shrink-0",
                                            value === campaign.id
                                                ? "opacity-100"
                                                : "opacity-0"
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
