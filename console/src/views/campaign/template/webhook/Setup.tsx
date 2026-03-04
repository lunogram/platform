import { Controller, useForm, useFieldArray, type UseFormReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus, Trash2 } from "lucide-react";
import type { Campaign, Template, User, Locale } from "@/types";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useContext, useState, useEffect } from "react";
import { ProjectContext, TemplateContext } from "@/contexts";
import api from "@/api";
import * as z from "zod";

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { UserSelection } from "../UserSelection";

const methods = ["DELETE", "GET", "PATCH", "POST", "PUT"] as const;

const webhookSetupFormSchema = z.object({
    method: z.enum(methods),
    endpoint: z.string().min(1, "Endpoint is required"),
    body: z.string().optional(),
    headers: z.array(z.object({
        key: z.string(),
        value: z.string(),
    })).optional(),
    cache_key: z.string().optional(),
});

type WebhookFormValues = z.infer<typeof webhookSetupFormSchema>;

function headersToArray(headers?: Record<string, string>): { key: string; value: string }[] {
    if (!headers || Object.keys(headers).length === 0) {
        return [{ key: "", value: "" }];
    }
    return Object.entries(headers).map(([key, value]) => ({ key, value }));
}

function bodyToString(body?: Record<string, any>): string {
    if (!body || Object.keys(body).length === 0) return "";
    try {
        return JSON.stringify(body, null, 2);
    } catch {
        return "";
    }
}

export function WebhookForm(_campaign: Campaign, template?: Template) {
    return useForm<WebhookFormValues>({
        resolver: zodResolver(webhookSetupFormSchema),
        defaultValues: {
            method: template?.data.method ?? "POST",
            endpoint: template?.data.endpoint ?? "",
            body: bodyToString(template?.data.body),
            headers: headersToArray(template?.data.headers),
            cache_key: template?.data.cache_key ?? "",
        },
    });
}

interface WebhookFormControlProps {
    campaign: Campaign;
    form: UseFormReturn<WebhookFormValues>;
    disabled?: boolean;
}

export function WebhookFormControl({ form, disabled = false }: WebhookFormControlProps) {
    const { t } = useTranslation();
    const { fields, append, remove } = useFieldArray({
        control: form.control,
        name: "headers",
    });

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="method"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel>{t('method')}</FieldLabel>
                        <Select
                            value={field.value}
                            onValueChange={field.onChange}
                            disabled={disabled}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {methods.map((m) => (
                                    <SelectItem key={m} value={m}>{m}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="endpoint"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel>{t('endpoint')}</FieldLabel>
                        <Input
                            {...field}
                            aria-invalid={fieldState.invalid}
                            placeholder="https://api.example.com/webhook"
                            autoComplete="off"
                            disabled={disabled}
                            readOnly={disabled}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />

            <Field className="gap-2">
                <div className="flex items-center justify-between">
                    <FieldLabel>{t('campaign.setup.channels.webhook.headers.label')}</FieldLabel>
                    {!disabled && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => append({ key: "", value: "" })}
                        >
                            <Plus className="w-4 h-4 mr-1" />
                            {t('campaign.setup.channels.webhook.headers.add')}
                        </Button>
                    )}
                </div>
                <div className="space-y-2">
                    {fields.map((item, index) => (
                        <div key={item.id} className="flex items-center gap-2">
                            <Controller
                                name={`headers.${index}.key`}
                                control={form.control}
                                render={({ field }) => (
                                    <Input
                                        {...field}
                                        placeholder="Header name"
                                        autoComplete="off"
                                        disabled={disabled}
                                        readOnly={disabled}
                                        className="flex-1"
                                    />
                                )}
                            />
                            <Controller
                                name={`headers.${index}.value`}
                                control={form.control}
                                render={({ field }) => (
                                    <Input
                                        {...field}
                                        placeholder="Value"
                                        autoComplete="off"
                                        disabled={disabled}
                                        readOnly={disabled}
                                        className="flex-1"
                                    />
                                )}
                            />
                            {!disabled && fields.length > 1 && (
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    onClick={() => remove(index)}
                                    className="shrink-0"
                                >
                                    <Trash2 className="w-4 h-4 text-muted-foreground" />
                                </Button>
                            )}
                        </div>
                    ))}
                </div>
            </Field>

            <Controller
                name="body"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel>{t('body')}</FieldLabel>
                        <Textarea
                            {...field}
                            aria-invalid={fieldState.invalid}
                            placeholder='{"key": "value"}'
                            autoComplete="off"
                            disabled={disabled}
                            readOnly={disabled}
                            className="font-mono text-sm min-h-[120px]"
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />

            <Controller
                name="cache_key"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel>{t('cache_key')}</FieldLabel>
                        <Input
                            {...field}
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                            disabled={disabled}
                            readOnly={disabled}
                        />
                        <p className="text-xs text-muted-foreground">{t('cache_key_subtitle')}</p>
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
        </FieldGroup>
    );
}

export interface WebhookSetupProps {
    campaign: Campaign;
    form: UseFormReturn<WebhookFormValues>;
    edit?: boolean;
}

const methodColors: Record<string, string> = {
    GET: "bg-emerald-600 text-white",
    POST: "bg-blue-600 text-white",
    PUT: "bg-amber-600 text-white",
    PATCH: "bg-orange-600 text-white",
    DELETE: "bg-red-600 text-white",
};

function buildCurlCommand(method: string, endpoint: string, headers: { key: string; value: string }[], body?: string): string {
    const parts: string[] = ["curl"];

    if (method !== "GET") {
        parts.push(`-X ${method}`);
    }

    parts.push(`'${endpoint || "https://..."}'`);

    const validHeaders = headers.filter(h => h.key.trim());
    for (const header of validHeaders) {
        parts.push(`-H '${header.key}: ${header.value}'`);
    }

    if (body && body.trim() && method !== "GET" && method !== "DELETE") {
        parts.push(`-d '${body}'`);
    }

    return parts.join(" \\\n  ");
}

export function WebhookPreview({ campaign, form, edit = false }: WebhookSetupProps) {
    const [project] = useContext(ProjectContext);
    const [template, setTemplate] = useContext(TemplateContext);
    const { t } = useTranslation();
    const [selectedUser, setSelectedUser] = useState<User | null>(null);
    const [selectedLocale, setSelectedLocale] = useState(template.locale);
    const [locales, setLocales] = useState<Locale[]>([]);
    const navigate = useNavigate();

    useEffect(() => {
        const fetchLocales = async () => {
            if (project?.id) {
                const result = await api.locales.search(project.id, { limit: 100 });
                setLocales(result.results);
            }
        };
        fetchLocales();
    }, [project?.id]);

    const method = form.watch("method") ?? "POST";
    const endpoint = form.watch("endpoint") ?? "";
    const headers = form.watch("headers") ?? [];
    const body = form.watch("body") ?? "";

    const curlCommand = buildCurlCommand(method, endpoint, headers, body);

    const handleEditTemplate = () => {
        navigate(`/projects/${project?.id}/campaigns/${campaign.id}/templates/${template.id}`);
    };

    const handleLocaleChange = async (locale: string) => {
        setSelectedLocale(locale);
        const newTemplate = campaign.templates.find(t => t.locale === locale);
        if (!newTemplate) {
            return;
        }
        setTemplate(newTemplate);
    };

    return (
        <>
            <div className="mb-4 flex items-center justify-between gap-4">
                <div className="flex-1">
                    <UserSelection
                        projectId={project?.id}
                        value={selectedUser}
                        onChange={setSelectedUser}
                    />
                </div>
                {edit && (
                    <div className="flex items-center gap-2">
                        <Select value={selectedLocale} onValueChange={handleLocaleChange}>
                            <SelectTrigger className="w-[180px]">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {campaign.templates.map((t) => {
                                    const locale = locales.find(l => l.key === t.locale);
                                    return (
                                        <SelectItem key={t.id} value={t.locale}>
                                            {locale?.label || t.locale}
                                        </SelectItem>
                                    );
                                })}
                            </SelectContent>
                        </Select>
                        <Button onClick={handleEditTemplate}>
                            {t('campaign.webhook.edit')}
                        </Button>
                    </div>
                )}
            </div>
            <div className="w-full">
                <div className="bg-zinc-800/90 rounded-lg shadow-lg overflow-hidden border border-zinc-600/50">
                    <div className="flex items-center gap-2 px-4 py-2.5 bg-zinc-700/60 border-b border-zinc-600/40">
                        <div className="flex gap-1.5">
                            <div className="w-2.5 h-2.5 rounded-full bg-red-400/80" />
                            <div className="w-2.5 h-2.5 rounded-full bg-yellow-400/80" />
                            <div className="w-2.5 h-2.5 rounded-full bg-green-400/80" />
                        </div>
                        <div className="flex-1 flex items-center gap-2 ml-2">
                            <Badge className={`${methodColors[method] ?? "bg-zinc-600 text-white"} text-[10px] px-1.5 py-0 font-bold rounded border-0`}>
                                {method}
                            </Badge>
                            <span className="text-zinc-400 text-xs font-mono truncate">
                                {endpoint || "https://..."}
                            </span>
                        </div>
                    </div>
                    <pre className="px-5 py-5 text-sm font-mono text-green-400/90 whitespace-pre-wrap break-all overflow-auto min-h-[200px] max-h-[500px] leading-relaxed">
                        <span className="text-zinc-500">$ </span>{curlCommand}
                    </pre>
                </div>
            </div>
        </>
    );
}
