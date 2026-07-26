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
import { fetchPathSuggestions } from "@/lib/path-suggestions"
import type { VariableSuggestions } from "@/types"

interface CampaignVariableContextValue {
    /** Variable groups available for campaign template editors */
    variableGroups: VariableGroup[]
    /**
     * Whether the schema fetch has settled, so `variableGroups` is as complete
     * as it will get. Until then the groups hold only the hardcoded base
     * variables and none of the project's own attributes.
     *
     * Editors that read their configuration once, at mount, must wait for this:
     * mounting early silently pins them to the base set for the rest of the
     * session. Set on failure as well as success — a project whose schema
     * endpoints are down still needs a usable editor.
     */
    variablesReady: boolean
}

const CampaignVariableContext = createContext<CampaignVariableContextValue>({
    variableGroups: [],
    variablesReady: false,
})

// eslint-disable-next-line react-refresh/only-export-components
export function useCampaignVariableContext() {
    return useContext(CampaignVariableContext)
}

// These are always available regardless of user schema or campaign
// variables. They are resolved server-side at send time.
const emailLinkVariables: VariableGroup = {
    label: "Links",
    variables: [
        {
            path: "unsubscribe_url",
            label: "Unsubscribe URL",
            description: "Link to unsubscribe",
            types: ["string"],
        },
        {
            path: "preferences_url",
            label: "Preferences URL",
            description: "Link to email preferences",
            types: ["string"],
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
            types: ["string"],
        },
        {
            path: "now | date: '%Y'",
            label: "Current Year",
            description: "Current year",
            types: ["string"],
        },
    ],
}

// Base user properties that are always available at send time,
// regardless of whether user schema data has been ingested.
const baseUserVariables: VariableGroup = {
    label: "User",
    variables: [
        { path: "user.id", label: "ID", description: "User ID", types: ["string"] },
        { path: "user.email", label: "Email", description: "Email address", types: ["string"] },
        { path: "user.phone", label: "Phone", description: "Phone number", types: ["string"] },
        {
            path: "user.external_id",
            label: "External ID",
            description: "External identifier",
            types: ["string"],
        },
        {
            path: "user.anonymous_id",
            label: "Anonymous ID",
            description: "Anonymous identifier",
            types: ["string"],
        },
        {
            path: "user.timezone",
            label: "Timezone",
            description: "User timezone",
            types: ["string"],
        },
        { path: "user.locale", label: "Locale", description: "User locale", types: ["string"] },
        {
            path: "user.created_at",
            label: "Created At",
            description: "Account creation date",
            types: ["date"],
        },
    ],
}

export function CampaignVariableProvider({ children }: PropsWithChildren) {
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const [suggestions, setSuggestions] = useState<VariableSuggestions | null>(null)
    const [variablesReady, setVariablesReady] = useState(false)

    // Fetch user schema paths once per project
    useEffect(() => {
        let cancelled = false
        setVariablesReady(false)
        fetchPathSuggestions(project.id)
            .then((s) => {
                if (!cancelled) setSuggestions(s)
            })
            .catch(console.error)
            .finally(() => {
                if (!cancelled) setVariablesReady(true)
            })
        return () => {
            cancelled = true
        }
    }, [project.id])

    const variableGroups = useMemo<VariableGroup[]>(() => {
        const groups: VariableGroup[] = []

        // Always include base user properties. Merge in any dynamic
        // schema paths (e.g. user.data.first_name) from the API on top.
        const basePaths = new Set(baseUserVariables.variables.map((v) => v.path))
        const userVariables = [...baseUserVariables.variables]

        if (suggestions?.userPaths?.length) {
            for (const p of suggestions.userPaths) {
                const cleanPath = p.path.replace(/^\./, "")
                const fullPath = `user.${cleanPath}`
                if (!basePaths.has(fullPath)) {
                    userVariables.push({
                        path: fullPath,
                        label: cleanPath,
                        description: p.types.join(", "),
                        types: p.types,
                    })
                }
            }
        }

        groups.push({ label: "User", variables: userVariables })

        if (campaign.variables?.length) {
            groups.push({
                label: "Campaign",
                variables: campaign.variables.map((v) => ({
                    path: `campaign.${v.name}`,
                    label: v.name,
                    description: v.default ? `default: ${v.default}` : undefined,
                    types: ["string"],
                    defaultValue: v.default,
                })),
            })
        }

        // Link variables are only relevant for email campaigns
        if (campaign.channel === "email") {
            groups.push(emailLinkVariables)
        }

        groups.push(otherSystemVariables)

        return groups
    }, [suggestions, campaign.variables, campaign.channel])

    const value = useMemo<CampaignVariableContextValue>(
        () => ({ variableGroups, variablesReady }),
        [variableGroups, variablesReady],
    )

    return (
        <CampaignVariableContext.Provider value={value}>
            {children}
        </CampaignVariableContext.Provider>
    )
}
