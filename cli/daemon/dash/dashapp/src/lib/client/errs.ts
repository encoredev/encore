export class ResponseError extends Error {
  path: string;
  code: string;
  detail: any | null;

  constructor(path: string, code: string, detail: any | null, message: string) {
    let d = detail ? JSON.stringify(detail) : "(no details)";
    if (message) {
        super(`API ${path} failed with code: ${code} - ${d}: ${message}`)
    } else {
        super(`API ${path} failed with code: ${code} - ${d}`)
    }
    this.path = path;
    this.code = code;
    this.detail = detail;
  }
}

export function errCode(err: any): string {
  if (!err || !err.code) {
    return "unknown"
  }
  return err.code ?? "unknown"
}

export function errDetail(err: any): any | null {
  if (!err || !err.detail) {
    return null
  }
  return err.detail || null
}

export function isErr(err: any, code?: string, detail?: any) {
    if (!err) {
      return false;
    } else if (err.code !== code) {
      return false;
    }

    if (!detail) {
      return true;
    } else if (!err.detail) {
      return false;
    }

    for (let key in detail) {
      if (detail[key] !== err.detail[key]) {
        return false;
      }
    }
    return true;
}

export function isValidationErr(err: any, field?: string, type?: string) {
  if (type) {
    return isErr(err, "validation", {"field": field, "type": type})
  } else if (field) {
    return isErr(err, "validation", {"field": field})
  } else {
    return isErr(err, "validation")
  }
}