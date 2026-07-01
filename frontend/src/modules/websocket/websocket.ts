let ws: WebSocket | null = null

type MessageHandler = (msg: string) => void
type StatusHandler = (status: "open" | "close" | "error") => void

const messageHandlers: MessageHandler[] = []
const statusHandlers: StatusHandler[] = []

export function connect(url = `${import.meta.env.VITE_WEBSOCKET_URL}`): void {
    ws = new WebSocket(url)

    ws.onopen = () => {
        console.log("WebSocket Connected")
        statusHandlers.forEach(h => h("open"))
    }

    ws.onmessage = (event: MessageEvent) => {
        messageHandlers.forEach(h => h(event.data))
    }

    ws.onclose = () => {
        console.log("WebSocket Closed, retrying...")
        statusHandlers.forEach(h => h("close"))
        setTimeout(() => connect(url), 1000)
    }

    ws.onerror = () => {
        statusHandlers.forEach(h => h("error"))
    }
}

export function sendMessage(message: string): void {
    if (ws?.readyState === WebSocket.OPEN) {
        ws.send(message)
    } else {
        console.warn("WebSocket not ready")
    }
}

export function onMessage(handler: MessageHandler): void {
    messageHandlers.push(handler)
}

export function onStatus(handler: StatusHandler): void {
    statusHandlers.push(handler)
}

export function getSocket(): WebSocket | null {
    return ws
}
