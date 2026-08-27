let ws: WebSocket | null = null
let currentUserId: string | null = null

type MessageHandler = (msg: string) => void
type StatusHandler = (status: "open" | "close" | "error") => void

const messageHandlers: MessageHandler[] = []
const statusHandlers: StatusHandler[] = []

// cryptography stub for insecure HTTP hosts
if (typeof window !== 'undefined' && window.crypto && !window.crypto.randomUUID) {
    // @ts-ignore
    window.crypto.randomUUID = function() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
            const r = (Math.random() * 16) | 0;
            const v = c === 'x' ? r : (r & 0x3) | 0x8;
            return v.toString(16);
        })
    }
    console.log('HTTP UUID Polyfill injected successfully')
}

export function connect(connectionId?: string, baseUrl = `${import.meta.env.VITE_WEBSOCKET_URL}`): void {
    currentUserId = connectionId || crypto.randomUUID()

    const urlWithParams = `${baseUrl}?userId=${encodeURIComponent(currentUserId)}`

    ws = new WebSocket(urlWithParams)

    ws.onopen = () => {
        console.log("WebSocket Connected. ID:", currentUserId)
        statusHandlers.forEach(h => h("open"))
    }

    ws.onmessage = (event: MessageEvent) => {
        messageHandlers.forEach(h => h(event.data))
    }

    ws.onclose = () => {
        console.log("WebSocket Closed, retrying...")
        statusHandlers.forEach(h => h("close"))
        setTimeout(() => connect(currentUserId!, baseUrl), 1000)
    }

    ws.onerror = () => {
        statusHandlers.forEach(h => h("error"))
    }
}

export function getUserId(): string | null {
    return currentUserId
}

export function sendMessage(message: string): void {
    if (ws?.readyState === WebSocket.OPEN) {
        ws.send(message)
    } else {
        console.warn("WebSocket not ready")
    }
}

export function onMessage(handler: MessageHandler): () => void {
    messageHandlers.push(handler)
    // Return a function that filters this specific handler out
    return () => {
        const index = messageHandlers.indexOf(handler);
        if (index !== -1) {
            messageHandlers.splice(index, 1);
        }
    }
}

export function onStatus(handler: StatusHandler): void {
    statusHandlers.push(handler)
}

export function getSocket(): WebSocket | null {
    return ws
}
