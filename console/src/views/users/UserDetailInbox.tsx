import { useContext } from "react"
import { UserContext } from "../../contexts"
import InboxDetailTable from "@/components/inbox-detail-table"

export default function UserDetailInbox() {
    const [user] = useContext(UserContext)

    return <InboxDetailTable subjectId={user.id} subjectType="users" />
}
