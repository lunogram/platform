import { useNavigate, useParams } from "react-router"

export const useRoute = (includeProject = true) => {
    const { projectId = "" } = useParams()
    const navigate = useNavigate()
    const parts: string[] = []
    if (includeProject) {
        parts.push("projects", projectId)
    }
    return (path: string) => {
        const newParts = [...parts, path]
        navigate("/" + newParts.join("/"))?.catch((e) => {
            console.error("Failed to navigate to:", e)
        })
    }
}
