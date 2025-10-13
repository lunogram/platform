import { useTranslation } from 'react-i18next'
import { EmailTemplateData, Provider, TemplateUpdateParams } from '../../../types'
import { useContext } from 'react'
import { TemplateContext } from '../../../contexts'
import { InfoTable, Tag } from '../../../ui'
import { UseFormReturn } from 'react-hook-form'
import TextInput from '../../../ui/form/TextInput'

const getFieldDisplay = ({
    field,
    value,
    defaultValue,
    required = true,
    t,
}: {
    field: string
    value?: string
    defaultValue?: string
    required?: boolean
    t: (key: string) => string
}) => {
    const effective = value ?? defaultValue
    if (!effective && required) {
        return <Tag variant="warn">{t('missing')}</Tag>
    }

    const invalidEmail = ['cc', 'bcc', 'reply_to', 'from_email'].includes(field) && effective && !effective.includes('@')
    if (invalidEmail) {
        return (
            <Tag variant="warn">
                {t('invalid_email')}: &quot;{effective}&quot;
            </Tag>
        )
    }

    return (
        <>
            {effective}
        </>
    )
}

export const EmailTable = ({ data, provider }: { data: EmailTemplateData, provider: Provider }) => {
    const { t } = useTranslation()
    const { currentTemplate, variants } = useContext(TemplateContext)

    return <>
        <InfoTable rows={{
            ...variants.length ? { [t('variant')]: currentTemplate?.name } : {},
            [t('from_email')]: getFieldDisplay({
                field: 'from_email',
                value: data.from?.address,
                defaultValue: provider.data?.default_from,
                t,
            }),
            [t('from_name')]: getFieldDisplay({
                field: 'from_name',
                value: data.from?.name,
                defaultValue: provider.data?.default_from_name,
                t,
            }),
            [t('reply_to')]: getFieldDisplay({
                field: 'reply_to',
                value: data.reply_to,
                defaultValue: provider.data?.default_reply_to,
                required: false,
                t,
            }),
            [t('cc')]: getFieldDisplay({
                field: 'cc',
                value: data.cc,
                required: false,
                t,
            }),
            [t('bcc')]: getFieldDisplay({
                field: 'bcc',
                value: data.bcc,
                required: false,
                t,
            }),
            [t('subject')]: getFieldDisplay({
                field: 'subject',
                value: data.subject,
                t,
            }),
            [t('preheader')]: data.preheader,
        }} />
    </>
}

export const EmailForm = ({ form, provider }: { form: UseFormReturn<TemplateUpdateParams, any>, provider: Provider }) => {
    const { t } = useTranslation()
    return <>
        <TextInput.Field
            form={form}
            name="data.from.name"
            placeholder={provider.data?.default_from_name}
            label={t('from_name')}
            required={!provider.data?.default_from_name} />
        <TextInput.Field
            form={form}
            name="data.from.address"
            placeholder={provider.data?.default_from}
            label={t('from_email')}
            type="email"
            required={!provider.data?.default_from} />
        <TextInput.Field
            form={form}
            name="data.subject"
            label={t('subject')}
            textarea
            required />
        <TextInput.Field
            form={form}
            name="data.preheader"
            label={t('preheader')}
            textarea />
        <TextInput.Field
            form={form}
            name="data.reply_to"
            placeholder={provider.data?.default_reply_to}
            label={t('reply_to')} />
        <TextInput.Field form={form} name="data.cc" label={t('cc')} />
        <TextInput.Field form={form} name="data.bcc" label={t('bcc')} />
    </>
}
