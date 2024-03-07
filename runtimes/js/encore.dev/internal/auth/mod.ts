import {getCurrentRequest} from "../reqtrack/mod";

export function getAuthData<T>(): T | null {
    const authData = getCurrentRequest()?.getAuthData();
    if (!authData) {
        return null;
    } else {
        return authData as T;
    }
}
