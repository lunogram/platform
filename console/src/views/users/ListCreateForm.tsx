import { useContext } from 'react'
import { useForm } from 'react-hook-form'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import type { List, ListCreateParams } from '../../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import { useTranslation } from 'react-i18next'
import { createWrapperRule } from './rules/RuleHelpers'

interface ListCreateFormProps {
    onCreated?: (list: List) => void
}

export function ListCreateForm({ onCreated }: ListCreateFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const form = useForm<ListCreateParams>({
        defaultValues: {
            name: '',
            type: 'dynamic',
            rule: createWrapperRule(),
        },
    })

    const onSubmit = async (values: ListCreateParams) => {
        const rule = values.rule ?? createWrapperRule()
        const created = await api.lists.create(project.id, {
            ...values,
            rule: values.type === 'dynamic' ? rule : undefined,
            is_visible: true,
        })
        onCreated?.(created)
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                <FormField
                    control={form.control}
                    name="name"
                    rules={{ required: true }}
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel className="inline-flex gap-1">
                                {t('create_list')}
                                <span className="text-destructive">*</span>
                            </FormLabel>
                            <FormControl>
                                <Input {...field} />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />

                <FormField
                    control={form.control}
                    name="type"
                    render={({ field }) => (
                        <FormItem className="space-y-3">
                            <FormLabel>{t('type')}</FormLabel>
                            <RadioGroup
                                onValueChange={field.onChange}
                                defaultValue={field.value}
                                className="flex w-full gap-2"
                            >
                                <div className="flex-1">
                                    <RadioGroupItem
                                        value="dynamic"
                                        id="dynamic"
                                        className="peer sr-only"
                                    />
                                    <label
                                        htmlFor="dynamic"
                                        className="flex items-center justify-center w-full px-4 py-2 rounded-md border bg-card text-sm font-medium cursor-pointer transition-colors hover:bg-muted peer-data-[state=checked]:bg-primary peer-data-[state=checked]:text-primary-foreground peer-data-[state=checked]:border-primary peer-data-[state=checked]:hover:bg-primary/90"
                                    >
                                        {t('dynamic')}
                                    </label>
                                </div>
                                <div className="flex-1">
                                    <RadioGroupItem
                                        value="static"
                                        id="static"
                                        className="peer sr-only"
                                    />
                                    <label
                                        htmlFor="static"
                                        className="flex items-center justify-center w-full px-4 py-2 rounded-md border bg-card text-sm font-medium cursor-pointer transition-colors hover:bg-muted peer-data-[state=checked]:bg-primary peer-data-[state=checked]:text-primary-foreground peer-data-[state=checked]:border-primary peer-data-[state=checked]:hover:bg-primary/90"
                                    >
                                        {t('static')}
                                    </label>
                                </div>
                            </RadioGroup>
                        </FormItem>
                    )}
                />

                <Button type="submit">{t('save')}</Button>
            </form>
        </Form>
    )
}
