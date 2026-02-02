import { createContext } from 'react';

export interface TemplateWorkflowContextValue {
    onSubmit: (fn: () => Promise<boolean> | boolean) => () => void;
    submit: () => Promise<void>;
}

export const TemplateWorkflowContext = createContext<TemplateWorkflowContextValue>({
    onSubmit: () => () => { },
    submit: async () => { },
});
