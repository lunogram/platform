import { createContext } from "react";

export interface CampaignDetailContextValue {
  onNext: (fn: () => Promise<boolean> | boolean) => () => void;
  next: () => Promise<void>;
}

export const CampaignDetailContext = createContext<CampaignDetailContextValue>({
  onNext: () => () => false,
  next: async () => { },
});