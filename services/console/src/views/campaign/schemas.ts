import * as z from "zod";

export const SetupBaseFormSchema = z.object({
    name: z.string().min(1, "Campaign name is required"),
    provider_id: z.string().min(1, "Provider is required"),
});

