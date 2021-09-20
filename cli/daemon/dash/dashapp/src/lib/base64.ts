export type Base64EncodedBytes = string;

// Unicode-compatible base64 encode/decode helpers, from
// https://developer.mozilla.org/en-US/docs/Glossary/Base64#the_unicode_problem

export function decodeBase64(str: string): string {
    return decodeURIComponent(escape(atob(str)))
}

export function encodeBase64(str: string): string {
    return btoa(unescape(encodeURIComponent(str)))
}
