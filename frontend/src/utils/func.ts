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

export const formatDynamicTime = (totalSeconds: number) => {
  if (!totalSeconds || isNaN(totalSeconds)) return '00:00'

  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = Math.floor(totalSeconds % 60)

  const paddedMinutes = String(minutes).padStart(2, '0')
  const paddedSeconds = String(seconds).padStart(2, '0')

  // If over an hour, prepend hours with a leading zero
  if (hours > 0) {
    const paddedHours = String(hours).padStart(2, '0');
    return `${paddedHours}:${paddedMinutes}:${paddedSeconds}`
  }

  return `${paddedMinutes}:${paddedSeconds}`
}
