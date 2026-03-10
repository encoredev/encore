# tsparser-wasm

WebAssembly build of the Encore TypeScript parser. Parses Encore.ts source files in the browser and returns application metadata (APIs, services, infrastructure resources, etc.).

## Prerequisites

- [Rust toolchain](https://rustup.rs/)
- [`wasm-pack`](https://rustwasm.github.io/wasm-pack/installer/)

```sh
cargo install wasm-pack
```

## Building

From this directory (`tsparser/wasm/`):

```sh
wasm-pack build --target web
```

This produces a `pkg/` directory containing:
- `tsparser_wasm_bg.wasm` — the compiled WebAssembly module
- `tsparser_wasm.js` — JavaScript bindings (ES module)
- `tsparser_wasm.d.ts` — TypeScript type declarations
- `package.json` — ready to publish to npm or use locally

## Usage

```js
import init, { parse } from './pkg/tsparser_wasm.js';

await init();

const files = [
  // User source files
  { name: "myservice/encore.service.ts", content: `import { Service } from "encore.dev/service";\nexport default new Service("myservice");` },
  { name: "myservice/api.ts", content: `import { api } from "encore.dev/api";\nexport const ping = api({ method: "GET", path: "/ping" }, async () => { return { message: "pong" }; });` },

  // Optional: tsconfig.json for path alias support
  { name: "tsconfig.json", content: `{"compilerOptions": {"paths": {"@/*": ["./src/*"]}}}` },

  // Optional: node_modules files (package.json + .d.ts type declarations)
  { name: "node_modules/zod/package.json", content: `{"name": "zod", "exports": {".": {"types": "./lib/index.d.mts"}}}` },
  { name: "node_modules/zod/lib/index.d.mts", content: `export declare function string(): ZodString; ...` },
];

const result = JSON.parse(parse(JSON.stringify(files)));

if (result.ok) {
  console.log("Parsed metadata:", result.meta);
} else {
  console.error("Parse errors:", result.errors);
}
```

### Input format

`parse()` accepts a JSON string containing an array of `{name, content}` objects:

| File pattern | Treatment |
|---|---|
| `node_modules/**` | Registered for module resolution but not parsed as user code. `package.json` files are used for bare import resolution (`exports`, `types`, `main` fields). |
| `tsconfig.json` | Parsed for `compilerOptions.paths` and `baseUrl` to support path aliases. JSONC (comments) is supported. |
| Everything else | Parsed as user source code. Only `.ts` files are processed. |

### Output format

Returns a JSON string:

```json
{
  "ok": true,
  "errors": [],
  "meta": { /* encore.parser.meta.v1 protobuf as JSON */ }
}
```

When `ok` is `false`, `errors` contains human-readable error messages and `meta` is absent.
