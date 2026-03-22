import { useContext } from "react"
import { OrganizationContext } from "../../contexts"
import ScheduledDetailTable from "@/components/scheduled-detail-table"

export default function OrganizationDetailScheduled() {
    const [organization] = useContext(OrganizationContext)

    return <ScheduledDetailTable subjectId={organization.id} subjectType="organizations" />
}
