import { useContext } from 'react'
import Preview from '@/ui/Preview'
import type { Campaign } from '@/types'
import { SingleSelect } from '@/ui/form/SingleSelect'
import { Heading, LinkButton } from '@/ui'
import { TemplateContext } from '@/contexts'
import { useTranslation } from 'react-i18next'

const JourneyTemplatePreview = ({ campaign }: { campaign: Campaign }) => {
    const { t } = useTranslation()
    const { variants, locales, currentLocale, currentTemplate, setTemplate, setLocale } = useContext(TemplateContext)
    return <>
        <Heading
            title={t('preview')}
            size="h4"
            actions={
                <>
                    {variants.length > 1 && <SingleSelect
                        options={variants}
                        size="small"
                        value={currentTemplate}
                        onChange={(variant) => setTemplate(variant)}
                    />}
                    <SingleSelect
                        options={locales}
                        size="small"
                        value={currentLocale}
                        onChange={(locale) => setLocale(locale)}
                    />
                    <LinkButton
                        to={`/projects/${campaign.project_id}/campaigns/${campaign.id}`}
                        size="small"
                        target="_blank"
                    >
                        {t('edit_campaign')}
                    </LinkButton>
                </>
            }
        />
        {currentTemplate && <Preview template={currentTemplate} />}
    </>
}

export default JourneyTemplatePreview