import { useTranslation } from "react-i18next"

export function UnsupportedChannel() {
    const { t } = useTranslation()
    return (
        <div className="flex flex-1 items-center justify-center bg-muted/20">
            <p className="text-muted-foreground">
                {t("campaign.channel_unsupported", "This channel type is not yet supported.")}
            </p>
        </div>
    )
}
