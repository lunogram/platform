import { ErrorSet, ErrorType } from '../core/errors'

export default {
    CampaignDoesNotExist: {
        message: 'The requested campaign does not exist or you do not have access.',
        code: 2000,
        statusCode: 400,
    },
    CampaignFinished: {
        message: 'The campaign has already finished and cannot be modified.',
        code: 2001,
        statusCode: 400,
    },
    CampaignInvalidProvider: {
        message: 'The provider for this campaign has been archived and can no longer be used.',
        code: 2002,
        statusCode: 400,
    },
} satisfies ErrorSet<ErrorType>
