import { createContext } from "react"

export interface CampaignWorkflowContextValue {
    onSubmit: (fn: () => Promise<boolean> | boolean) => () => void
    submit: () => Promise<void>
}

export const CampaignWorkflowContext = createContext<CampaignWorkflowContextValue>({
    onSubmit: () => () => {},
    submit: async () => {},
})
