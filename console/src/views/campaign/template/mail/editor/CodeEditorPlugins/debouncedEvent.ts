export default class DebouncedEvent {
    private timeout: ReturnType<typeof setTimeout> | null = null

    public trigger<ArgType extends unknown[], T extends (...args: ArgType) => void>(
        func: T,
        args: ArgType,
        delay = 400,
    ) {
        if (this.timeout) {
            clearTimeout(this.timeout)
        }

        this.timeout = setTimeout(() => {
            func(...args)

            this.timeout = null
        }, delay)
    }
}
