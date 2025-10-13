import './i18n'
import App from './App'
import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'
import reportWebVitals from './reportWebVitals'
import { ClerkProvider } from '@clerk/clerk-react'

import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/700.css'
import './variables.css'
import './index.css'

const CLERK_PUBLISHABLE_KEY = process.env.REACT_APP_CLERK_PUBLISHABLE_KEY

const root = ReactDOM.createRoot(
    document.getElementById('root') as HTMLElement,
)
root.render(
    <StrictMode>
        {CLERK_PUBLISHABLE_KEY
            ? (
                <ClerkProvider publishableKey={CLERK_PUBLISHABLE_KEY}>
                    <App />
                </ClerkProvider>
            )
            : (
                <App />
            )}
    </StrictMode>,
)

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals()
