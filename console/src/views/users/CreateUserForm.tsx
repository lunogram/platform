import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import type { User } from '../../types'
import { Input } from '@/components/ui/input'
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'

interface CreateUserFormProps {
    defaultUser: Pick<User, 'timezone' | 'locale' | 'data'>
    timeZones: string[]
    onSubmit: (user: User) => Promise<void>
}

export function CreateUserForm({ defaultUser, timeZones, onSubmit }: CreateUserFormProps) {
    const { t } = useTranslation()
    const [isLoading, setIsLoading] = useState(false)
    const form = useForm<User>({
        defaultValues: defaultUser,
    })

    const handleSubmit = async (data: User) => {
        if (isLoading) return
        setIsLoading(true)
        try {
            await onSubmit(data)
        } catch (error: unknown) {
            console.error('Error creating user', error)
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-4">
                <FormField
                    control={form.control}
                    name="full_name"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t('full_name')}</FormLabel>
                            <FormControl>
                                <Input {...field} />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name="email"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t('email')}</FormLabel>
                            <FormControl>
                                <Input {...field} type="email" />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name="phone"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t('phone')}</FormLabel>
                            <FormControl>
                                <Input {...field} type="tel" placeholder="+31612345678" />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name="timezone"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t('timezone')}</FormLabel>
                            <Select onValueChange={field.onChange} defaultValue={field.value}>
                                <FormControl>
                                    <SelectTrigger>
                                        <SelectValue placeholder={t('timezone')} />
                                    </SelectTrigger>
                                </FormControl>
                                <SelectContent side="bottom" className="max-h-[200px]">
                                    {timeZones.map((tz) => (
                                        <SelectItem key={tz} value={tz}>
                                            {tz}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name="locale"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t('locale.singular')}</FormLabel>
                            <FormControl>
                                <Input {...field} />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <Button type="submit" className="w-full" disabled={isLoading}>
                    {isLoading ? t('loading') : t('create')}
                </Button>
            </form>
        </Form>
    )
}
