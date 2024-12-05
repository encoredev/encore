import * as runtime from "../internal/runtime/mod";

export class IterableStream {
  private stream: runtime.Stream;

  constructor(stream: runtime.Stream) {
    this.stream = stream;
  }

  recv(): Promise<Record<string, any>> {
    return this.stream.recv();
  }

  async *[Symbol.asyncIterator]() {
    while (true) {
      try {
        yield await this.stream.recv();
      } catch (e) {
        break;
      }
    }
  }
}

export class IterableSocket {
  private socket: runtime.Socket;

  constructor(socket: runtime.Socket) {
    this.socket = socket;
  }

  send(msg: Record<string, any>): void {
    return this.socket.send(msg);
  }
  recv(): Promise<Record<string, any>> {
    return this.socket.recv();
  }

  close(): void {
    this.socket.close();
  }

  async *[Symbol.asyncIterator]() {
    while (true) {
      try {
        yield await this.socket.recv();
      } catch (e) {
        break;
      }
    }
  }
}

export class Sink {
  private sink: runtime.Sink;

  constructor(sink: runtime.Sink) {
    this.sink = sink;
  }

  send(msg: Record<string, any>): void {
    return this.sink.send(msg);
  }

  close(): void {
    this.sink.close();
  }
}
