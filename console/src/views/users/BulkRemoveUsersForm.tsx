import { useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import { Button } from '@/components/ui/button'

interface BulkRemoveUsersFormProps {
    onSubmit: (file: FileList) => Promise<void>
}

export function BulkRemoveUsersForm({ onSubmit }: BulkRemoveUsersFormProps) {
    const { t } = useTranslation()
    const form = useForm<{ file: FileList }>()
    const [fileName, setFileName] = useState<string>('')
    const fileInputRef = useRef<HTMLInputElement>(null)

    return (
        <Form {...form}>
            <form
                onSubmit={form.handleSubmit(async (data) => {
                    await onSubmit(data.file)
                })}
                className="space-y-5"
            >
                <div className="rounded-lg bg-amber-50 dark:bg-amber-950/20 border border-amber-200 dark:border-amber-800 p-4">
                    <p className="text-sm leading-relaxed text-amber-900 dark:text-amber-200">
                        {t('delete_users_instructions')}
                    </p>
                </div>
                <FormField
                    control={form.control}
                    name="file"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t('file')}</FormLabel>
                            <FormControl>
                                <div className="flex items-center gap-3">
                                    <Input
                                        ref={fileInputRef}
                                        type="file"
                                        className="sr-only"
                                        onChange={(e) => {
                                            field.onChange(e.target.files)
                                            setFileName(e.target.files?.[0]?.name || '')
                                        }}
                                        required
                                        accept=".csv"
                                    />
                                    <Button
                                        type="button"
                                        variant="outline"
                                        onClick={() => fileInputRef.current?.click()}
                                    >
                                        {t('upload')}
                                    </Button>
                                    <span className="text-sm text-muted-foreground">
                                        {fileName}
                                    </span>
                                </div>
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <Button type="submit" variant="destructive" className="w-full">
                    {t('delete')}
                </Button>
            </form>
        </Form>
    )
}
