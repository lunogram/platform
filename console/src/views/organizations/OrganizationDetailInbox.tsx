import { useContext } from "react"
import { OrganizationContext } from "../../contexts"
import InboxDetailTable from "@/components/inbox-detail-table"

export default function OrganizationDetailInbox() {
    const [organization] = useContext(OrganizationContext)

    return <InboxDetailTable subjectId={organization.id} subjectType="organizations" />
}
