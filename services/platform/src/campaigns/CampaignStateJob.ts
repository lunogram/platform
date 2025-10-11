import { subDays } from 'date-fns'
import { Job } from '../queue'
import Campaign from './Campaign'
import { CacheKeys, updateCampaignProgress } from './CampaignService'
import App from '../app'

export default class CampaignStateJob extends Job {
    static $name = 'campaign_state_job'

    static from(): CampaignStateJob {
        return new this().deduplicationKey(this.$name)
    }

    static async handler() {

        // Fetch anything that is currently running, has finished
        // within the last two days or has activity since last run
        const openedCampaignIds = await App.main.redis.smembers(CacheKeys.pendingStats).then(ids => ids.map(parseInt).filter(x => x))
        const campaigns = await Campaign.all(qb => qb
            .whereIn('state', ['loading', 'scheduled', 'running'])
            .orWhere(function(qb) {
                qb.where('state', 'finished')
                    .where('send_at', '>', subDays(Date.now(), 2))
            })
            .orWhere(function(qb) {
                qb.whereIn('id', openedCampaignIds)
                    .whereNotIn('state', ['draft', 'aborted'])
            }),
        )

        for (const campaign of campaigns) {
            await updateCampaignProgress(campaign)
        }

        if (campaigns.length) {
            await App.main.redis.srem(CacheKeys.pendingStats, ...campaigns.map(c => c.id))
        }
    }
}
