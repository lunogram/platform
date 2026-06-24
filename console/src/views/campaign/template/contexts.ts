import { createContext } from "react"

export interface TemplateWorkflowContextValue {
    onSubmit: (fn: () => Promise<boolean> | boolean) => () => void
    submit: () => Promise<void>
    save: () => Promise<boolean>
    setCanProceed: (canProceed: boolean) => void
    canProceed: boolean
}

export const TemplateWorkflowContext = createContext<TemplateWorkflowContextValue>({
    onSubmit: () => () => {},
    submit: async () => {},
    save: async () => false,
    setCanProceed: () => {},
    canProceed: true,
})
