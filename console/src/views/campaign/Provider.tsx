import { useTranslation } from "react-i18next"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"

export default function CampaignProvider() {
    const { t } = useTranslation()

    return (
        <div className="flex flex-1 items-center justify-center bg-muted/20">
            <div className="w-full max-w-md space-y-6 bg-background p-8 rounded-lg border">
                <div className="space-y-2">
                    <h1 className="text-2xl font-semibold">{t("campaign.provider.title")}</h1>
                    <p className="text-muted-foreground">{t("campaign.provider.description")}</p>
                </div>

                <Field className="gap-2">
                    <FieldLabel>{t("campaign.provider.form.provider.label")}</FieldLabel>
                    <div className="text-sm text-muted-foreground">
                        Provider selection component will be added here
                    </div>
                    <FieldDescription>
                        {t("campaign.provider.form.provider.description")}
                    </FieldDescription>
                </Field>
            </div>
        </div>
    )
}
