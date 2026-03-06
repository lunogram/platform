class CodeStore {
    private code = ""

    public get current() {
        return this.code
    }

    setCode(next: string) {
        if (next === this.code) return
        this.code = next
    }
}

export default new CodeStore()
