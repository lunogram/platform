import api from '../../api'
import type { Project } from '../../types'
import { useTranslation } from 'react-i18next'
import type { UseFormReturn } from 'react-hook-form'
import { Controller, FormProvider, useForm } from 'react-hook-form'
import {
    Globe,
    MessageSquareText,
    Link2,
} from 'lucide-react'

import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Separator } from "@/components/ui/separator"
import { LocalePicker } from '@/components/locale/picker'

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
    const browserLocale = navigator.languages[0] ?? 'en'
    const defaults = project ?? {
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        locale: browserLocale,
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
        }

        try {
            const updatedProject = id
                ? await api.projects.update(id, params)
                : await api.projects.create(params)
            onSave?.(updatedProject)
        } catch (error) {
            console.error('Failed to save project', error)
            window.alert(
                t('project.saveError', 'Unable to save project. Please try again.')
            )
        }
    })

    const isEditing = !!project
    return (
        <FormProvider {...form}>
            <form onSubmit={handleSubmit} className="space-y-6">

                {isEditing ? (
                    <>
                        {/* Project Details — settings page layout */}
                        <section className="space-y-6">
                            <div className="flex items-center gap-3">
                                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
                                    <Globe className="h-4 w-4 text-primary" />
                                </div>
                                <div>
                                    <h3 className="font-semibold leading-none tracking-tight">{t('project_details', 'Project Details')}</h3>
                                    <p className="text-sm text-muted-foreground">
                                        {t('project_details_description', 'Basic information about your project.')}
                                    </p>
                                </div>
                            </div>

                            <div className="grid gap-6">

                                <div className="grid gap-x-4 gap-y-2 sm:grid-cols-2">
                                    <Label htmlFor="name" className="inline-flex items-center gap-1">
                                        {t('name')} <span className="text-destructive">*</span>
                                    </Label>
                                    <Label className="inline-flex items-center gap-1">
                                        {t('default_locale')} <span className="text-destructive">*</span>
                                    </Label>

                                    <Controller
                                        control={form.control}
                                        name="name"
                                        rules={{ required: true }}
                                        render={({ field }) => (
                                            <Input
                                                id="name"
                                                type="text"
                                                required
                                                placeholder={t('project_name_placeholder', 'My Project')}
                                                value={field.value ?? ''}
                                                onChange={field.onChange}
                                                onBlur={field.onBlur}
                                                ref={field.ref}
                                            />
                                        )}
                                    />
                                    <Controller
                                        control={form.control}
                                        name="locale"
                                        rules={{ required: true }}
                                        render={({ field }) => (
                                            <LocalePicker
                                                value={field.value}
                                                onChange={field.onChange}
                                            />
                                        )}
                                    />

                                    <span />
                                    <p className="text-xs text-muted-foreground">
                                        {t('default_locale_description')}
                                    </p>
                                </div>

                                <div className="grid gap-2">
                                    <Label htmlFor="description">
                                        {t('description')}
                                    </Label>
                                    <Controller
                                        control={form.control}
                                        name="description"
                                        render={({ field }) => (
                                            <Textarea
                                                id="description"
                                                placeholder={t('project_description_placeholder', 'A brief description of your project...')}
                                                className="min-h-[80px] resize-none"
                                                value={field.value ?? ''}
                                                onChange={field.onChange}
                                                onBlur={field.onBlur}
                                                ref={field.ref}
                                            />
                                        )}
                                    />
                                </div>

                                <div className="grid gap-2 sm:max-w-sm">
                                    <Label htmlFor="timezone" className="inline-flex items-center gap-1">
                                        {t('timezone')} <span className="text-destructive">*</span>
                                    </Label>
                                    <Controller
                                        control={form.control}
                                        name="timezone"
                                        rules={{ required: true }}
                                        render={({ field }) => (
                                            <Select value={field.value} onValueChange={field.onChange}>
                                                <SelectTrigger id="timezone">
                                                    <SelectValue placeholder={t('timezone')} />
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
                                </div>

                            </div>
                        </section>

                        <ProjectSettingsFields form={form} />
                    </>
                ) : (
                    /* Onboarding / create project — simple stacked layout */
                    <div className="grid gap-4">
                        <div className="grid gap-2">
                            <Label htmlFor="name" className="inline-flex items-center gap-1">
                                {t('name')} <span className="text-destructive">*</span>
                            </Label>
                            <Controller
                                control={form.control}
                                name="name"
                                rules={{ required: true }}
                                render={({ field }) => (
                                    <Input
                                        id="name"
                                        type="text"
                                        required
                                        placeholder={t('project_name_placeholder', 'My Project')}
                                        value={field.value ?? ''}
                                        onChange={field.onChange}
                                        onBlur={field.onBlur}
                                        ref={field.ref}
                                    />
                                )}
                            />
                        </div>

                        <div className="grid gap-2">
                            <Label htmlFor="description">
                                {t('description')}
                            </Label>
                            <Controller
                                control={form.control}
                                name="description"
                                render={({ field }) => (
                                    <Textarea
                                        id="description"
                                        placeholder={t('project_description_placeholder', 'A brief description of your project...')}
                                        className="min-h-[80px] resize-none"
                                        value={field.value ?? ''}
                                        onChange={field.onChange}
                                        onBlur={field.onBlur}
                                        ref={field.ref}
                                    />
                                )}
                            />
                        </div>

                        <div className="grid gap-2">
                            <Label className="inline-flex items-center gap-1">
                                {t('default_locale')} <span className="text-destructive">*</span>
                            </Label>
                            <Controller
                                control={form.control}
                                name="locale"
                                rules={{ required: true }}
                                render={({ field }) => (
                                    <LocalePicker
                                        value={field.value}
                                        onChange={field.onChange}
                                    />
                                )}
                            />
                            <p className="text-xs text-muted-foreground">
                                {t('default_locale_description')}
                            </p>
                        </div>

                        <div className="grid gap-2">
                            <Label htmlFor="timezone" className="inline-flex items-center gap-1">
                                {t('timezone')} <span className="text-destructive">*</span>
                            </Label>
                            <Controller
                                control={form.control}
                                name="timezone"
                                rules={{ required: true }}
                                render={({ field }) => (
                                    <Select value={field.value} onValueChange={field.onChange}>
                                        <SelectTrigger id="timezone">
                                            <SelectValue placeholder={t('timezone')} />
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
                        </div>
                    </div>
                )}

                {/* Save */}
                <div className="flex items-center justify-end">
                    <Button type="submit" disabled={form.formState.isSubmitting}>
                        {isEditing ? t('save') : t('create_project')}
                    </Button>
                </div>

            </form>
        </FormProvider>
    )
}

export function ProjectSettingsFields({ form }: { form: UseFormReturn<Project> }) {
    const { t } = useTranslation()
    return <>

        <Separator />

        {/* Message Settings */}
        <section className="space-y-6">
            <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
                    <MessageSquareText className="h-4 w-4 text-primary" />
                </div>
                <div>
                    <h3 className="font-semibold leading-none tracking-tight">{t('message_settings')}</h3>
                    <p className="text-sm text-muted-foreground">
                        {t('message_settings_description', 'Configure automatic reply messages for SMS interactions.')}
                    </p>
                </div>
            </div>

            <div className="grid gap-6">

                <div className="grid gap-2">
                    <Label htmlFor="text_opt_out_message">
                        {t('sms_opt_out_message')}
                    </Label>
                    <Controller
                        control={form.control}
                        name="text_opt_out_message"
                        render={({ field }) => (
                            <Input
                                id="text_opt_out_message"
                                type="text"
                                placeholder={t('sms_opt_out_message_placeholder', 'You have been unsubscribed...')}
                                value={field.value || ''}
                                onChange={field.onChange}
                                onBlur={field.onBlur}
                            />
                        )}
                    />
                    <p className="text-xs text-muted-foreground">
                        {t('sms_opt_out_message_subtitle')}
                    </p>
                </div>

                <div className="grid gap-2">
                    <Label htmlFor="text_help_message">
                        {t('sms_help_message')}
                    </Label>
                    <Controller
                        control={form.control}
                        name="text_help_message"
                        render={({ field }) => (
                            <Input
                                id="text_help_message"
                                type="text"
                                placeholder={t('sms_help_message_placeholder', 'Reply STOP to unsubscribe...')}
                                value={field.value || ''}
                                onChange={field.onChange}
                                onBlur={field.onBlur}
                            />
                        )}
                    />
                    <p className="text-xs text-muted-foreground">
                        {t('sms_help_message_subtitle')}
                    </p>
                </div>

            </div>
        </section>

        <Separator />

        {/* Link Wrapping */}
        <section className="space-y-6">
            <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
                    <Link2 className="h-4 w-4 text-primary" />
                </div>
                <div>
                    <h3 className="font-semibold leading-none tracking-tight">{t('link_wrapping', 'Link Wrapping')}</h3>
                    <p className="text-sm text-muted-foreground">
                        {t('link_wrapping_description', 'Enable link tracking for different channels.')}
                    </p>
                </div>
            </div>

            <div className="grid gap-4">

                <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                    <div className="space-y-0.5">
                        <Label htmlFor="link_wrap_email" className="text-sm font-medium">
                            {t('link_wrapping_email')}
                        </Label>
                        <p className="text-xs text-muted-foreground">
                            {t('link_wrapping_email_subtitle')}
                        </p>
                    </div>
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

                <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                    <div className="space-y-0.5">
                        <Label htmlFor="link_wrap_push" className="text-sm font-medium">
                            {t('link_wrapping_push')}
                        </Label>
                        <p className="text-xs text-muted-foreground">
                            {t('link_wrapping_push_subtitle')}
                        </p>
                    </div>
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

            </div>
        </section>
    </>
}
