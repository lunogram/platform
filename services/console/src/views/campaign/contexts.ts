import { createContext } from "react";

export interface CampaignDetailContextValue {
  onNext: (fn: () => Promise<void> | void) => () => void;
  next: () => Promise<void>;
}

export const CampaignDetailContext = createContext<CampaignDetailContextValue>({
  onNext: () => () => { },
  next: async () => { },
});