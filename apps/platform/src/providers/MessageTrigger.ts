import { UUID } from "node:crypto"

export interface MessageTrigger {
    campaign_id: UUID
    user_id: UUID
    event_id?: UUID
    reference_type?: string
    reference_id?: UUID
}
