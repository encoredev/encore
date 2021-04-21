export type Base64EncodedBytes = string;

export function decodeBase64(str: string): string {
    return atob(str)
}

export function encodeBase64(str: string): string {
    return btoa(str)
}
