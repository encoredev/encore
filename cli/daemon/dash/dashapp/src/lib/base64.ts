export type Base64EncodedBytes = string;

export function decodeBase64(str: string): string {
    const raw = atob(str)
    return decodeURIComponent(raw.split('').map(function(c) {
        return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)
    }).join(''))
}

export function encodeBase64(str: string): string {
    let encoded = encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, function(match, p1) {
        return String.fromCharCode(('0x' + p1) as any)
    })
    return btoa(encoded)
}
