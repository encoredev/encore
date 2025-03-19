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

// InOut stream type variants
export const inOutWithHandshake = api.streamInOut<Handshake, InMsg, OutMsg>(
  { expose: true, path: "/inout/:pathParam" },
  async (handshake: Handshake, stream) => {},
);
export const inOutWithoutHandshake = api.streamInOut<InMsg, OutMsg>(
  { expose: true, path: "/inout/noHandshake" },
  async (stream) => {},
);

// Out stream type variants
export const outWithHandshake = api.streamOut<Handshake, OutMsg>(
  { expose: true, path: "/out/:pathParam" },
  async (handshake: Handshake, stream) => {},
);

export const outWithoutHandshake = api.streamOut<OutMsg>(
  { expose: true, path: "/out/noHandshake" },
  async (stream) => {},
);

// In stream type variants
export const inWithHandshake = api.streamIn<Handshake, InMsg>(
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

export const inWithResponseAndHandshake = api.streamIn<Handshake, InMsg, OutMsg>(
  { expose: true, path: "/in/withResponseAndHandshake" },
  async (handshake: Handshake, stream) => {},
);
