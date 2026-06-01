import * as z from "zod"

export const deviceFormSchema = z
    .object({
        device_id: z.string().min(1, "Device ID is required"),
        os: z.enum(["ios", "android", "web"]),
        token: z.string().optional(),
        endpoint: z.string().optional(),
        auth_key: z.string().optional(),
        p256dh_key: z.string().optional(),
        os_version: z.string().optional(),
        model: z.string().optional(),
        app_build: z.string().optional(),
        app_version: z.string().optional(),
    })
    .superRefine((data, ctx) => {
        if (data.os === "web") {
            if (!data.endpoint) {
                ctx.addIssue({
                    code: "custom",
                    message: "Endpoint is required",
                    path: ["endpoint"],
                })
            }
            if (!data.auth_key) {
                ctx.addIssue({
                    code: "custom",
                    message: "Auth key is required",
                    path: ["auth_key"],
                })
            }
            if (!data.p256dh_key) {
                ctx.addIssue({
                    code: "custom",
                    message: "P256DH key is required",
                    path: ["p256dh_key"],
                })
            }
        } else {
            if (!data.token) {
                ctx.addIssue({ code: "custom", message: "Token is required", path: ["token"] })
            }
        }
    })

export type DeviceFormValues = z.infer<typeof deviceFormSchema>
