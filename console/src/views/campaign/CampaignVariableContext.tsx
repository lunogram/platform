import {
    createContext,
    useContext,
    useEffect,
    useMemo,
    useState,
    type PropsWithChildren,
} from "react"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"
import { CampaignContext, ProjectContext } from "@/contexts"
import api from "@/api"
import type { VariableSuggestions } from "@/types"

// ── Context value ───────────────────────────────────────────────────

interface CampaignVariableContextValue {
    /** Variable groups available for campaign template editors */
    variableGroups: VariableGroup[]
}

const CampaignVariableContext = createContext<CampaignVariableContextValue>({
    variableGroups: [],
})

// eslint-disable-next-line react-refresh/only-export-components
export function useCampaignVariableContext() {
    return useContext(CampaignVariableContext)
}

// ── System variable groups ──────────────────────────────────────────
// These are always available regardless of user schema or campaign
// variables. They are resolved server-side at send time.

const emailLinkVariables: VariableGroup = {
    label: "Links",
    variables: [
        {
            path: "unsubscribe_url",
            label: "Unsubscribe URL",
            description: "Link to unsubscribe",
        },
        {
            path: "preferences_url",
            label: "Preferences URL",
            description: "Link to email preferences",
        },
    ],
}

const campaignSystemVariables: VariableGroup = {
    label: "Campaign",
    variables: [
        {
            path: "campaign.name",
            label: "Campaign Name",
            description: "Name of the campaign",
        },
    ],
}

const otherSystemVariables: VariableGroup = {
    label: "Other",
    variables: [
        {
            path: "now | date",
            label: "Current Date",
            description: "Current date",
        },
        {
            path: "now | date: '%Y'",
            label: "Current Year",
            description: "Current year",
        },
    ],
}

// ── Provider ────────────────────────────────────────────────────────

export function CampaignVariableProvider({ children }: PropsWithChildren) {
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [suggestions, setSuggestions] = useState<VariableSuggestions | null>(null)

    // Fetch user schema paths once per project
    useEffect(() => {
        let cancelled = false
        api.projects
            .pathSuggestions(project.id)
            .then((s) => {
                if (!cancelled) setSuggestions(s)
            })
            .catch(console.error)
        return () => {
            cancelled = true
        }
    }, [project.id])

    const variableGroups = useMemo<VariableGroup[]>(() => {
        const groups: VariableGroup[] = []

        // ── User properties ─────────────────────────────────
        if (suggestions?.userPaths?.length) {
            groups.push({
                label: "User",
                variables: suggestions.userPaths.map((p) => {
                    const cleanPath = p.path.replace(/^\./, "")
                    return {
                        path: `user.${cleanPath}`,
                        label: cleanPath,
                        description: p.types.join(", "),
                    }
                }),
            })
        }

        // ── Campaign-defined variables ──────────────────────
        if (campaign.variables?.length) {
            groups.push({
                label: "Campaign Variables",
                variables: campaign.variables.map((v) => ({
                    path: v.name,
                    label: v.name,
                    description: v.default ? `default: ${v.default}` : undefined,
                })),
            })
        }

        // ── System variables ────────────────────────────────
        // Link variables are only relevant for email campaigns
        if (campaign.channel === "email") {
            groups.push(emailLinkVariables)
        }

        groups.push(campaignSystemVariables)
        groups.push(otherSystemVariables)

        return groups
    }, [suggestions, campaign.variables, campaign.channel])

    const value = useMemo<CampaignVariableContextValue>(
        () => ({ variableGroups }),
        [variableGroups],
    )

    return (
        <CampaignVariableContext.Provider value={value}>
            {children}
        </CampaignVariableContext.Provider>
    )
}
