let ws: WebSocket | null = null
let currentUserId: string | null = null

type MessageHandler = (msg: string) => void
type StatusHandler = (status: "open" | "close" | "error") => void

const messageHandlers: MessageHandler[] = []
const statusHandlers: StatusHandler[] = []

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
