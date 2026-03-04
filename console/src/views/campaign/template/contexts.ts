import { createContext } from 'react';

export interface TemplateWorkflowContextValue {
    onSubmit: (fn: () => Promise<boolean> | boolean) => () => void;
    submit: () => Promise<void>;
    setCanProceed: (canProceed: boolean) => void;
    canProceed: boolean;
}

export const TemplateWorkflowContext = createContext<TemplateWorkflowContextValue>({
    onSubmit: () => () => { },
    submit: async () => { },
    setCanProceed: () => { },
    canProceed: true,
});
