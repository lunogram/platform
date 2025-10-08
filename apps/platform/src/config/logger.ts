import pino from 'pino'
import pretty from 'pino-pretty'
import ErrorHandler, { ErrorConfig } from '../error/ErrorHandler'

export type LoggerConfig = {
    level: string
    prettyPrint: boolean
    error: ErrorConfig
}

export const logger = pino({
    level: process.env.LOG_LEVEL || 'warn',
}, process.env.LOG_PRETTY_PRINT ? pretty({ colorize: true }) : undefined)

export default async (config: LoggerConfig) => {
    logger.level = config.level
    return new ErrorHandler(config.error)
}
