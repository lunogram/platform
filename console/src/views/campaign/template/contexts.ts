import { createContext } from "react"
import { t } from "i18next"
import { toast } from "sonner"
import { wasReported } from "@/lib/query"
import { problemDetail } from "@/utils"

/**
 * The outcome of saving a template step: "invalid" means the form already
 * shows what to fix, "failed" means the request failed and the user was told.
 */
export type SaveResult = { ok: true } | { ok: false; reason: "invalid" | "failed" }

/** Persists a template step's edits. Never throws. */
export type SaveStep = () => Promise<SaveResult>

/** The save of a step with no edits of its own to persist. */
export const nothingToSave: SaveStep = async () => ({ ok: true })

/**
 * StepSaveContext gives the parts of a step, such as a preview's send-test
 * button, the step's own save. Outside a template step (e.g. on the campaign
 * details page) there is nothing to save.
 */
export const StepSaveContext = createContext<SaveStep>(nothingToSave)

/**
 * @returns the result of running a save request. A failure is shown to the
 * user (unless the mutation cache already did) and returned, never thrown.
 */
export async function saveWith(request: () => Promise<unknown>): Promise<SaveResult> {
    try {
        await request()
        return { ok: true }
    } catch (err) {
        if (!wasReported(err)) {
            toast.error(
                problemDetail(err) ??
                    t("campaign.template.save_failed", "Couldn't save the template"),
            )
        }
        return { ok: false, reason: "failed" }
    }
}
