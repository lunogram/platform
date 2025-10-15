import { JsonViewer, JsonViewerOnChange } from '@textea/json-viewer'
import { useContext } from 'react'
import { PreferencesContext } from './PreferencesContext'

import './JsonPreview.css'

interface JsonPreviewParams {
    value: Record<string | number, any>
    editable?: boolean
    onChange?: JsonViewerOnChange
}

export default function JsonPreview({ value, editable, onChange }: JsonPreviewParams) {
    const [preferences] = useContext(PreferencesContext)
    return (
        <JsonViewer
            editable={editable}
            onChange={onChange}
            value={value}
            rootName={false}
            theme={preferences.mode}
        />
    )
}
