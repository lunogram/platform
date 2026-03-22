import { useContext } from "react"
import { UserContext } from "../../contexts"
import ScheduledDetailTable from "@/components/scheduled-detail-table"

export default function UserDetailScheduled() {
    const [user] = useContext(UserContext)

    return <ScheduledDetailTable subjectId={user.id} subjectType="users" />
}
