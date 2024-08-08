-- encore.app --
{"id": ""}

-- package.json --
{"name": "ts-test-app"}

-- svc/svc.ts --
import { api, Header, Query } from "encore.dev/api";

interface Handshake {
    headerValue: Header<"some-header">;
    queryValue: Query<"some-query">;
    pathParam: string;
}
interface InMsg {
    data: string;
}
interface OutMsg {
    user: number;
    msg: string;
}

// Bidi stream type variants
export const bidiWithHandshake = api.streamBidi<InMsg, OutMsg>(
  { expose: true, path: "/bidi/:pathParam" },
  async (handshake: Handshake, stream) => {},
);
export const bidiWithoutHandshake = api.streamBidi<InMsg, OutMsg>(
  { expose: true, path: "/bidi/noHandshake" },
  async (stream) => {},
);

// Out stream type variants
export const outWithHandshake = api.streamOut<OutMsg>(
  { expose: true, path: "/out/:pathParam" },
  async (handshake: Handshake, stream) => {},
);

export const outWithoutHandshake = api.streamOut<OutMsg>(
  { expose: true, path: "/out/noHandshake" },
  async (stream) => {},
);

// In stream type variants
export const inWithHandshake = api.streamIn<InMsg>(
  { expose: true, path: "/in/:pathParam" },
  async (handshake: Handshake, stream) => {},
);

export const inWithoutHandshake = api.streamIn<InMsg>(
  { expose: true, path: "/in/noHandshake" },
  async (stream) => {},
);

export const inWithResponse = api.streamIn<InMsg, OutMsg>(
  { expose: true, path: "/in/withResponse" },
  async (stream) => {},
);

export const inWithResponseAndHandshake = api.streamIn<InMsg, OutMsg>(
  { expose: true, path: "/in/withResponse" },
  async (handshake: Handshake, stream) => {},
);
