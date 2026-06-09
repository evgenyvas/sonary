export function sleep(ms: number): Promise<any> {
    return new Promise(resolve => setTimeout(resolve, ms))
}

export const merge = (a: any, b: any, predicate = (a: any, b: any) => a === b) => {
    const c = [...a] // copy to avoid side effects
    // add all items from B to copy C if they're not already present
    b.forEach((bItem: any) => (c.some((cItem) => predicate(bItem, cItem)) ? null : c.push(bItem)))
    return c
}

export const escapeHtml = (html: string) => {
    const div = document.createElement('div')
    div.textContent = html
    return div.innerHTML
}

export function partMap(parts: { [key: string]: boolean }) {
    return Object.entries(parts)
        .filter(([, value]) => value)
        .map(([key]) => key)
        .join(' ');
}
