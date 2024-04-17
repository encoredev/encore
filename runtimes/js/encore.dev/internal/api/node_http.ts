import type {
  IncomingHttpHeaders,
  OutgoingHttpHeader,
  OutgoingHttpHeaders,
} from "node:http";
import type { Socket } from "node:net";
import * as stream from "node:stream";
import * as runtime from "../runtime/mod";

export class RawRequest extends stream.Readable {
  complete: boolean;

  trailers: NodeJS.Dict<string>;
  trailersDistinct: NodeJS.Dict<string[]>;
  rawTrailers: string[];

  private body: runtime.BodyReader;
  private req: runtime.Request;

  constructor(req: runtime.Request, body: runtime.BodyReader) {
    super({});
    this.req = req;
    this.complete = false;

    this.trailers = {};
    this.trailersDistinct = {};
    this.rawTrailers = [];

    this.body = body;
    this.body.start(this.push.bind(this), this.destroy.bind(this));
  }

  get method(): string {
    return this.meta.apiCall!.method;
  }

  _url: string | undefined;
  get url(): string {
    if (!this._url) {
      this._url = this.meta.apiCall!.pathAndQuery;
    }
    return this._url;
  }
  set url(value: string) {
    this._url = value;
  }

  get headers(): IncomingHttpHeaders {
    return this.meta.apiCall!.headers;
  }

  _headersDistinct: NodeJS.Dict<string[]> | undefined;
  get headersDistinct(): NodeJS.Dict<string[]> {
    if (this._headersDistinct) {
      return this._headersDistinct;
    }

    const headers: NodeJS.Dict<string[]> = {};
    for (const [key, value] of Object.entries(this.headers)) {
      if (value !== undefined) {
        const val: string[] = Array.isArray(value) ? value : [value];
        headers[key] = val;
      }
    }
    this._headersDistinct = headers;
    return headers;
  }

  _rawHeaders: string[] | undefined;
  get rawHeaders(): string[] {
    if (this._rawHeaders) {
      return this._rawHeaders;
    }

    this._rawHeaders = Object.keys(this.headers);
    return this._rawHeaders;
  }

  private _meta: runtime.RequestMeta | undefined;
  private get meta(): runtime.RequestMeta {
    if (this._meta === undefined) {
      this._meta = this.req.meta();
    }
    return this._meta;
  }

  _read(size: number): void {
    this.body.read();
  }

  setTimeout(msecs: number, callback?: () => void): this {
    // Not yet implemented.
    return this;
  }
}

export class RawResponse extends stream.Writable {
  readonly req: RawRequest;
  chunkedEncoding: boolean;
  shouldKeepAlive: boolean;
  // useChunkedEncodingByDefault: boolean;
  sendDate: boolean;
  statusCode: number;
  statusMessage: string | undefined;

  finished: boolean; // deprecated
  headersSent: boolean;
  strictContentLength: boolean;

  readonly connection: Socket | null; // deprecated
  readonly socket: Socket | null;

  private w: runtime.ResponseWriter;
  private headers: OutgoingHttpHeaders;

  constructor(req: RawRequest, w: runtime.ResponseWriter) {
    super({ highWaterMark: 1024 * 1024 }); // TODO?
    this.req = req;
    this.chunkedEncoding = false; // TODO
    this.shouldKeepAlive = true;
    this.sendDate = true;
    this.statusCode = 200;
    this.statusMessage = undefined;
    this.finished = false;
    this.strictContentLength = false;
    this.headersSent = false;
    this.headers = {};

    this.connection = null;
    this.socket = null;
    this.w = w;
  }

  write(
    chunk: any,
    callback?: ((error: Error | null | undefined) => void) | undefined
  ): boolean;
  write(
    chunk: any,
    encoding: BufferEncoding,
    callback?: ((error: Error | null | undefined) => void) | undefined
  ): boolean;
  write(chunk: unknown, encoding?: unknown, callback?: unknown): boolean {
    const res = super.write(chunk, encoding as any, callback as any);
    // HACK: Work around pipe deadlock in Node.js when writing a large response.
    return true;
  }

  // Needed for Next.js compatibility.
  _implicitHeader() {
    this._writeHeaderIfNeeded();
  }

  _writeHeaderIfNeeded() {
    if (!this.headersSent) {
      this.w.writeHead(
        this.statusCode,
        this.headers as Record<string, string | number | string[]>
      );
      this.headersSent = true;
    }
  }

  _write(
    chunk: Buffer,
    _encoding: BufferEncoding,
    callback: (error?: Error | null) => void
  ) {
    this._writeHeaderIfNeeded();
    this.w.writeBody(chunk, callback);
  }

  _writev(
    chunks: Array<{ chunk: Buffer }>,
    callback: (error?: Error | null) => void
  ) {
    this._writeHeaderIfNeeded();
    this.w.writeBodyMulti(
      chunks.map((ch) => ch.chunk),
      callback
    );
  }

  _final(callback: (error?: Error | null | undefined) => void): void {
    this.w.close(undefined, callback);
  }

  setTimeout(msecs: number, callback?: () => void): this {
    // Not implemented yet.
    return this;
  }

  setHeader(name: string, value: number | string | string[]): this {
    this.headers[name] = value;
    return this;
  }

  appendHeader(name: string, value: number | string | string[]): this {
    const existing = this.headers[name];
    const existingIsArr = Array.isArray(existing);
    const valIsArr = Array.isArray(value);
    if (existingIsArr && valIsArr) {
      existing.push(...value);
    } else if (existingIsArr) {
      existing.push("" + value);
    } else if (existing !== undefined) {
      this.headers[name] = ["" + existing, "" + value];
    } else {
      this.headers[name] = value;
    }
    return this;
  }

  getHeader(name: string): number | string | string[] | undefined {
    return this.headers[name];
  }

  getHeaders(): OutgoingHttpHeaders {
    return this.headers;
  }

  getHeaderNames(): string[] {
    return Object.keys(this.headers);
  }

  hasHeader(name: string): boolean {
    return this.headers[name] !== undefined;
  }

  removeHeader(name: string): void {
    delete this.headers[name];
  }

  addTrailers(
    headers: OutgoingHttpHeaders | readonly [string, string][]
  ): void {
    // Not implemented yet.
  }

  flushHeaders(): void {
    this._writeHeaderIfNeeded();
  }

  writeHead(
    statusCode: number,
    headers?: OutgoingHttpHeaders | OutgoingHttpHeader[]
  ): this;
  writeHead(
    statusCode: number,
    statusMessage?: string,
    headers?: OutgoingHttpHeaders | OutgoingHttpHeader[]
  ): this;
  writeHead(
    statusCode: number,
    statusMessageOrHeaders?:
      | string
      | OutgoingHttpHeaders
      | OutgoingHttpHeader[],
    headers?: OutgoingHttpHeaders | OutgoingHttpHeader[]
  ): this {
    this.statusCode = statusCode;

    const headersIn =
      typeof statusMessageOrHeaders === "string"
        ? headers
        : statusMessageOrHeaders;

    // Merge headers, if provided.
    if (headersIn) {
      if (Array.isArray(headersIn)) {
        for (let i = 0; i < headersIn.length; i += 2) {
          const key = headersIn[i];
          const value = headersIn[i + 1];
          if (typeof key === "string" && typeof value === "string") {
            this.headers[key] = value;
          }
        }
      } else {
        for (const key in headersIn) {
          const value = headersIn[key];
          this.headers[key] = value;
        }
      }
    }

    this._writeHeaderIfNeeded();
    return this;
  }
}
