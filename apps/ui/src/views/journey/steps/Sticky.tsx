import { JourneyStepType } from '../../../types'
import { StickyStepIcon } from '../../../ui/icons'
import { useTranslation } from 'react-i18next'
import TextInput from '../../../ui/form/TextInput'

interface StickyConfig {
    text?: string
}

const TextAutoLink = ({ text }: { text: string }) => {
    const delimiter = /((?:https?:\/\/)?(?:(?:[a-z0-9]?(?:[a-z0-9\\-]{1,61}[a-z0-9])?\.[^\\.|\s])+[a-z\\.]*[a-z]+|(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3})(?::\d{1,5})*[a-z0-9.,_\\/~#&=;%+?\-\\(\\)]*)/gi
    return (
        <>
            {text.split(delimiter).map(word => {
                const match = word.match(delimiter)
                if (match) {
                    const url = match[0]
                    return (
                        <a key={url} target="_blank" href={url.startsWith('http') ? url : `https://${url}`} rel="noreferrer">{url}</a>
                    )
                }
                return word
            })}
        </>
    )
}

export const stickyStep: JourneyStepType<StickyConfig> = {
    name: 'sticky',
    icon: <StickyStepIcon />,
    category: 'info',
    description: 'sticky_desc',
    Describe({
        value,
    }) {
        return (
            <div style={{ maxWidth: 300 }}>
                <TextAutoLink text={value.text ?? ''} />
            </div>
        )
    },
    Edit({
        onChange,
        value,
    }) {
        const { t } = useTranslation()
        return (
            <TextInput
                name="sticky_text"
                label={t('sticky_text_label')}
                value={value.text ?? ''}
                onChange={(text) => onChange({ ...value, text })} // Update the text field
                textarea
            />
        )
    },
    hideBottomHandle: true,
    hideTopHandle: true,
}
