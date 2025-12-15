import { useContext } from 'react'
import { toast } from 'react-hot-toast/headless'
import api from '../../api'
import { ProjectContext } from '../../contexts'
import type { Journey } from '../../types'
import FormWrapper from '../../ui/form/FormWrapper'
import TextInput from '../../ui/form/TextInput'
import { TagPicker } from '../settings/TagPicker'
import { useTranslation } from 'react-i18next'
import RadioInput from '../../ui/form/RadioInput'
import { SingleSelect } from '../../ui/form/SingleSelect'

interface JourneyFormProps {
    journey?: Journey
    onSaved?: (journey: Journey) => void
}

export function JourneyForm({ journey, onSaved }: JourneyFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const statusOptions = [
        { key: 'live', label: t('live') },
        { key: 'off', label: t('off') },
    ]

    const templates = [
        { key: 'onboarding', label: 'Onboarding' },
    ]

    function templateToValue(option: { key: string, label: string }) {
        return option.key
    }

    const isCreated = journey?.id && journey?.status !== 'draft'
    return (
        <FormWrapper<Journey>
            onSubmit={async ({ id, name, description, status, template_id, tags }) => {
                const saved = id
                    ? await api.journeys.update(project.id, id, { name, description, status, tags })
                    : await api.journeys.create(project.id, { name, description, status, template_id, tags })
                toast.success(t('journey_saved'))
                onSaved?.(saved)
            }}
            defaultValues={journey}
            submitLabel={t('save')}
        >
            {
                form => (
                    <>
                        <TextInput.Field
                            form={form}
                            name="name"
                            label={t('name')}
                            required
                        />
                        <TextInput.Field
                            form={form}
                            name="description"
                            label={t('description')}
                            textarea
                        />
                        {(!isCreated && templates.length > 0) && <SingleSelect.Field
                            form={form}
                            options={templates}
                            toValue={templateToValue}
                            name="template_id"
                            label={t('template')}
                        />}
                        {/* <TagPicker.Field
                            form={form}
                            name="tags"
                            label={t('tags')}
                        /> */}
                        {isCreated && <RadioInput.Field
                            form={form}
                            name="status"
                            label={t('status')}
                            options={statusOptions}
                            required
                        />}
                    </>
                )
            }
        </FormWrapper>
    )
}
