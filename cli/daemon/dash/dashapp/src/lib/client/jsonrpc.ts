import { EventEmitter } from 'events';
import * as protocol from "json-rpc-protocol"

function makeAsync<T>(fn: (msg: Message) => T): (msg: Message) => Promise<T> {
  return function(msg) {
    return new Promise(function(resolve) {
      return resolve(fn(msg));
    });
  }
}

export interface RequestMsg {
  type: "request";
  id: number;
  method: string;
  params: any;
}

export interface ResponseMsg {
  type: "response";
  id: number;
  method: string;
  result: any;
}

export interface NotificationMsg {
  type: "notification";
  method: string;
  params: any;
}

export interface ErrorMsg {
  type: "error";
  id: number | null;
  error: {
    message: string;
    code: any;
    data: any;
  };
}

export type Message = RequestMsg | ResponseMsg | NotificationMsg | ErrorMsg;

export default class JSONRPCConn extends EventEmitter {
  _peer: Peer;
  _ws: WebSocket;

  constructor(ws: WebSocket) {
    super();
    this._ws = ws
    this._peer = new Peer(
      (msg) => ws.send(msg),
      (msg) => this.emit("notification", msg),
    )
    ws.onmessage = (event) => this._peer.processMsg(event.data);
  }

  async request<T>(method: string, params?: any): Promise<T> {
      return await this._peer.request<T>(method, params);
  }

  async notify(method: string, params?: any): Promise<void> {
    this._peer.notify(method, params)
  }

  close() {
    this._peer.failPendingRequests("closing connection")
    this._ws.close()
  }
}

const parseMessage = (message: string): Message => {
  try {
    return protocol.parse(message) as Message;
  } catch (error) {
    throw protocol.format.error(null, error);
  }
};

// Default onMessage implementation:
//
// - ignores notifications
// - throw MethodNotFound for all requests
function defaultOnMessage(message: Message) {
  if (message.type === "request") {
    throw new protocol.MethodNotFound(message.method);
  }
}

function noop() {}

interface Deferred {
  resolve: (x: any) => void;
  reject: (x: any) => void;
}

// Starts the autoincrement id with the JavaScript minimal safe integer to have
// more room before running out of integers (it's very far fetched but a very
// long running process with a LOT of messages could run out).
let nextRequestId = -9007199254740991;

// ===================================================================

export class Peer {
  _deferreds: {[key: number]: Deferred};
  _handle: (msg: Message) => Promise<any>;
  _send: (msg: string) => void;

  constructor(send: (msg: string) => void, onMessage = defaultOnMessage) {
    this._send = send;
    this._handle = makeAsync(onMessage);
    this._deferreds = Object.create(null);
  }

  _getDeferred(id: number): Deferred {
    const deferred = this._deferreds[id];
    delete this._deferreds[id];
    return deferred;
  }

  async processMsg(message: string) {
    const msg = parseMessage(message);

    if (msg.type === "error") {
      // Some errors do not have an identifier, simply discard them.
      if (msg.id === null) {
        return;
      }

      const { error } = msg;
      this._getDeferred(msg.id).reject(
        // TODO: it would be great if we could return an error with of
        // a more specific type (and custom types with registration).
        new (Error as any)(error.message, error.code, error.data)
      );
    } else if (msg.type === "response") {
      this._getDeferred(msg.id).resolve(msg.result);
    } else if (msg.type === "notification") {
      this._handle(msg).catch(noop);
    } else if (msg.type === "request") {
      return this._handle(msg)
        .then(result =>
          protocol.format.response(msg.id, result === undefined ? null : result)
        )
        .catch(error =>
          protocol.format.error(
            msg.id,

            // If the method name is not defined, default to the method passed
            // in the request.
            error instanceof protocol.MethodNotFound && !error.data
              ? new protocol.MethodNotFound(msg.method)
              : error
          )
        );
    }
  }

  // Fails all pending requests.
  failPendingRequests(reason: any) {
    Object.entries(this._deferreds).forEach(([id, deferred]) => {
      deferred.reject(reason);
      delete this._deferreds[(id as unknown) as number];
    });
  }

  /**
   * This function should be called to send a request to the other end.
   *
   * TODO: handle multi-requests.
   */
  request<T>(method: string, params: any): Promise<T> {
    return new Promise((resolve, reject) => {
      const requestId = nextRequestId++;

      try {
        this._send(protocol.format.request(requestId, method, params));
      } catch(err) {
        reject(err);
        return;
      }

      this._deferreds[requestId] = { resolve, reject };
    });
  }

  /**
   * This function should be called to send a notification to the other end.
   *
   * TODO: handle multi-notifications.
   */
  notify(method: string, params: any) {
    this._send(protocol.format.notification(method, params));
  }
}