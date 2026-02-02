declare global {
    interface Window { API_BASE_URL: string }
}

export const env = {
    api: {
        baseURL:
            window.API_BASE_URL || import.meta.env.VITE_API_BASE_URL || '/api',
    },
}
