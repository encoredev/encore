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
  headers: IncomingHttpHeaders;
  headersDistinct: NodeJS.Dict<string[]>;
  rawHeaders: string[];

  trailers: NodeJS.Dict<string>;
  trailersDistinct: NodeJS.Dict<string[]>;
  rawTrailers: string[];

  method: string;
  url: string;

  private body: runtime.BodyReader;

  constructor(req: runtime.Request, body: runtime.BodyReader) {
    super({}); // TODO?
    this.complete = false;
    this.headers = {};
    this.headersDistinct = {};
    this.rawHeaders = [];
    this.trailers = {};
    this.trailersDistinct = {};
    this.rawTrailers = [];

    // TODO
    this.method = req.method()!;
    this.url = req.path()!;
    this.body = body;
    this.body.start(this.push.bind(this), this.destroy.bind(this));
  }

  setTimeout(msecs: number, callback?: () => void): this {
    // TODO
    return this;
  }

  _read(size: number): void {
    this.body.read();
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
    super({}); // TODO?
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

  end(cb?: (() => void) | undefined): this;
  end(chunk: any, cb?: (() => void) | undefined): this;
  end(
    chunk: any,
    encoding: BufferEncoding,
    cb?: (() => void) | undefined
  ): this;
  end(chunk?: unknown, encoding?: unknown, cb?: unknown): this {
    let buffer: Buffer =
      chunk instanceof Buffer
        ? chunk
        : chunk instanceof Uint8Array
        ? Buffer.from(chunk)
        : Buffer.from(
            (chunk as string | undefined) ?? "",
            encoding as BufferEncoding
          );

    this._writeHeaderIfNeeded();
    this.w.close(buffer, cb as any);
    return this;
  }

  // Needed for Next.js compatibility.
  _implicitHeader() {
    this._writeHeaderIfNeeded();
  }

  _writeHeaderIfNeeded() {
    if (!this.headersSent) {
      // TODO headers
      this.w.writeHead(this.statusCode, {});
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

  _final(callback: (error?: Error | null | undefined) => void) {
    this.w.close(undefined, callback);
  }

  setTimeout(msecs: number, callback?: () => void): this {
    // TODO? Implement
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
    // NYI
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
    // TODO set headers
    this.w.writeHead(statusCode, {});
    return this;
  }

  // writeContinue(callback?: () => void): void;
  // writeEarlyHints(
  //   hints: Record<string, string | string[]>,
  //   callback?: () => void
  // ): void;
  // writeProcessing(): void;
}
