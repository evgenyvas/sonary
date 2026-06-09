import '@awesome.me/webawesome/dist/components/callout/callout.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import { escapeHtml } from './func'

// Custom function to emit toast notifications
export const notify = (message: string) => {
    return alert(escapeHtml(message))
}
