import api from '../../api'
import type { Project } from '../../types'
import { useTranslation } from 'react-i18next'
import type { UseFormReturn } from 'react-hook-form'
import { Controller, FormProvider, useForm } from 'react-hook-form'
import { useState } from 'react'
import { languageName } from '../../utils'

import {   
  Field,
  FieldDescription,
  FieldLabel,
  FieldLegend,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

// eslint-disable-next-line @typescript-eslint/no-namespace
export declare namespace Intl {
    type Key = 'calendar' | 'collation' | 'currency' | 'numberingSystem' | 'timeZone' | 'unit'
    function supportedValuesOf(input: Key): string[]

    interface DateTimeFormat {
         
        format(date?: Date | number): string
         
        resolvedOptions(): ResolvedDateTimeFormatOptions
    }

    interface ResolvedDateTimeFormatOptions {
        locale: string
        timeZone: string
        timeZoneName?: string
    }

    // eslint-disable-next-line no-var
    var DateTimeFormat: {
        new(locales?: string | string[]): DateTimeFormat
        (locales?: string | string[]): DateTimeFormat
    }
}

interface ProjectFormProps {
    project?: Project
    onSave?: (project: Project) => void
}

export default function ProjectForm({ project, onSave }: ProjectFormProps) {
    const { t } = useTranslation()
    const timeZones = Intl.supportedValuesOf('timeZone')
    const locale = navigator.languages[0]?.split('-')[0] ?? 'en'
    const defaults = project ?? {
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        locale,
        link_wrap_email: false,
        link_wrap_push: false,
    }
    const form = useForm<Project>({
        defaultValues: defaults,
    })
    const handleSubmit = form.handleSubmit(async ({
        id,
        name,
        description,
        locale,
        timezone,
        text_opt_out_message,
        text_help_message,
        link_wrap_email,
        link_wrap_push,
    }) => {
        const params = {
            name,
            description,
            locale,
            timezone,
            text_opt_out_message,
            text_help_message,
            link_wrap_email,
            link_wrap_push,
            tools: project?.tools,
        }

        const updatedProject = id
            ? await api.projects.update(id, params)
            : await api.projects.create(params)

        onSave?.(updatedProject)
    })

    const [language, setLanguage] = useState<string | undefined>(
        languageName(project?.locale ?? locale)
    )

    const isEditing = !!project
    return (
            <FormProvider {...form}>
                <form onSubmit={handleSubmit}>
                    <Field>
                        <FieldLabel htmlFor="name" className="flex gap-1">
                            {t('name')}
                            <span className="text-destructive">*</span>
                        </FieldLabel>
                        <Controller
                            control={form.control}
                            name="name"
                            rules={{ required: true }}
                            render={({ field }) => (
                                <Input
                                    id={t('name')}
                                    type="text"
                                    required
                                    value={field.value ?? ''}
                                    onChange={field.onChange}
                                    onBlur={field.onBlur}
                                    ref={field.ref}
                                />
                            )}
                        />
                    </Field>

                    <Field>
                        <FieldLabel htmlFor="description" className="mt-2">
                            {t('description')}
                        </FieldLabel>
                        <Controller
                            control={form.control}
                            name="description"
                            render={({ field }) => (
                                <Textarea
                                    id="description"
                                    value={field.value ?? ''}
                                    onChange={field.onChange}
                                    onBlur={field.onBlur}
                                    ref={field.ref}
                                />
                            )}
                        />
                    </Field>

                    <Field className="mt-5">
                        <FieldLegend className="text-2xl">{t('defaults')}</FieldLegend>
                    </Field>

                    <Field>
                        <FieldLabel htmlFor="locale" className="flex gap-1">
                            {t('default_locale')}
                            <span className="text-destructive">*</span>
                        </FieldLabel>
                        <FieldDescription className="text-xs">
                            {t('default_locale_description')}
                        </FieldDescription>
                        <div className="relative">
                            <Input
                                id="locale"
                                type="text"
                                {...form.register('locale', {
                                    onChange: (e) => setLanguage(languageName(e.target.value)),
                                    required: true,
                                })}
                            />
                            {language && (
                                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                                    {language}
                                </span>
                            )}
                        </div>
                    </Field>

                    <Field>
                        <FieldLabel htmlFor="timezone" className="flex gap-1 mt-2">
                            {t('timezone')}
                            <span className="text-destructive">*</span>
                        </FieldLabel>
                        <Controller
                            control={form.control}
                            name="timezone"
                            rules={{ required: true }}
                            render={({ field }) => (
                                <Select value={field.value} onValueChange={field.onChange}>
                                    <SelectTrigger id="timezone">
                                        <SelectValue placeholder={t('timezone_placeholder')} />
                                    </SelectTrigger>
                                    <SelectContent className="max-h-[300px]">
                                        <SelectGroup>
                                            {timeZones.map((tz) => (
                                                <SelectItem key={tz} value={tz}>
                                                    {tz}
                                                </SelectItem>
                                            ))}
                                        </SelectGroup>
                                    </SelectContent>
                                </Select>
                            )}
                        />
                    </Field>

                    {isEditing && <ProjectSettings form={form} />}

                    <div className="mt-6 flex justify-start">
                        <Button type="submit" disabled={form.formState.isSubmitting}>
                            {isEditing ? t('save') : t('create_project')}
                        </Button>
                    </div>
                </form>
            </FormProvider>
        )
}

export function ProjectSettings({ form }: { form: UseFormReturn<Project> }) {
    const { t } = useTranslation()
    return <>
        <Field className="mt-5">
            <FieldLegend className="text-2xl">{t('message_settings')}</FieldLegend>
        </Field>

        <Field>
            <FieldLabel htmlFor="text_opt_out_message" className="flex gap-1">
                {t('sms_opt_out_message')}
            </FieldLabel>
            <FieldDescription className="text-xs">
                {t('sms_opt_out_message_subtitle')}
            </FieldDescription>
            <Controller
                control={form.control}
                name="text_opt_out_message"
                render={({ field }) => (
                    <Input
                        id="text_opt_out_message"
                        type="text"
                        value={field.value || ''}
                        onChange={field.onChange}
                        onBlur={field.onBlur}
                    />
                )}
            />
        </Field>

        <Field>
            <FieldLabel htmlFor="text_help_message" className="mt-2">
                {t('sms_help_message')}
            </FieldLabel>
            <FieldDescription className="text-xs">
                {t('sms_help_message_subtitle')}
            </FieldDescription>
            <Controller
                control={form.control}
                name="text_help_message"
                render={({ field }) => (
                    <Input
                        id="text_help_message"
                        type="text"
                        value={field.value || ''}
                        onChange={field.onChange}
                        onBlur={field.onBlur}
                    />
                )}
            />
        </Field>

        <Field>
            <FieldLabel htmlFor="link_wrap_email" className="flex gap-1 mt-2 text-base">
                {t('link_wrapping_email')}
            </FieldLabel>
            <FieldDescription className="text-xs">
                {t('link_wrapping_email_subtitle')}
            </FieldDescription>
            <div className="w-fit">
                <Controller
                    control={form.control}
                    name="link_wrap_email"
                    render={({ field }) => (
                        <Switch
                            id="link_wrap_email"
                            checked={!!field.value}
                            onCheckedChange={field.onChange}
                        />
                    )}
                />
            </div>
        </Field>

        <Field>
            <FieldLabel htmlFor="link_wrap_push" className="flex gap-1 mt-2 text-base">
                {t('link_wrapping_push')}
            </FieldLabel>
            <FieldDescription className="text-xs">
                {t('link_wrapping_push_subtitle')}
            </FieldDescription>
            <div className="w-fit">
                <Controller
                    control={form.control}
                    name="link_wrap_push"
                    render={({ field }) => (
                        <Switch
                            id="link_wrap_push"
                            checked={!!field.value}
                            onCheckedChange={field.onChange}
                        />
                    )}
                />
            </div>
        </Field>
    </>
}