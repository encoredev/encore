import { readdir, readFile, writeFile, unlink, rename } from "node:fs/promises";
import { join } from "node:path";

const DOCS_DIR = new URL("../../../../docs/ts/runtime/", import.meta.url);

const RENAMES = {
  "mod.md": "encore-dev.md",
  "api.mod.md": "api.md",
  "auth.mod.md": "auth.md",
  "config.mod.md": "config.md",
  "cron.mod.md": "cron.md",
  "log.mod.md": "log.md",
  "metrics.mod.md": "metrics.md",
  "pubsub.mod.md": "pubsub.md",
  "service.mod.md": "service.md",
  "storage.cache.mod.md": "storage-cache.md",
  "storage.objects.mod.md": "storage-objects.md",
  "storage.sqldb.mod.md": "storage-sqldb.md",
  "types.mod.md": "types.md",
  "validate.mod.md": "validate.md"
};

const TITLES = {
  "index.md": "Runtime API Reference",
  "encore-dev.md": "encore.dev",
  "api.md": "encore.dev/api",
  "auth.md": "encore.dev/auth",
  "config.md": "encore.dev/config",
  "cron.md": "encore.dev/cron",
  "log.md": "encore.dev/log",
  "metrics.md": "encore.dev/metrics",
  "pubsub.md": "encore.dev/pubsub",
  "service.md": "encore.dev/service",
  "storage-cache.md": "encore.dev/storage/cache",
  "storage-objects.md": "encore.dev/storage/objects",
  "storage-sqldb.md": "encore.dev/storage/sqldb",
  "types.md": "encore.dev/types",
  "validate.md": "encore.dev/validate"
};

const dir = new URL(DOCS_DIR);

const files = await readdir(dir);

if (files.includes("index.md")) {
  await unlink(new URL("index.md", dir));
}

for (const [from, to] of Object.entries(RENAMES)) {
  if (files.includes(from)) {
    await rename(new URL(from, dir), new URL(to, dir));
  }
}

const finalFiles = await readdir(dir);
for (const file of finalFiles) {
  if (!file.endsWith(".md")) continue;
  const path = new URL(file, dir);
  let content = await readFile(path, "utf8");

  for (const [from, to] of Object.entries(RENAMES)) {
    const fromBase = from.replace(/\.md$/, "");
    const toBase = to.replace(/\.md$/, "");
    content = content.replaceAll(`/ts/runtime/${from}`, `/ts/runtime/${toBase}`);
    content = content.replaceAll(`/ts/runtime/${fromBase}`, `/ts/runtime/${toBase}`);
    content = content.replaceAll(`(${from})`, `(/ts/runtime/${toBase})`);
    content = content.replaceAll(`(./${from})`, `(/ts/runtime/${toBase})`);
  }

  content = content.replaceAll("(index.md)", "(/ts/runtime/)");
  content = content.replaceAll("(./index.md)", "(/ts/runtime/)");
  content = content.replace(/\((\/ts\/runtime\/[^)#]+)\.md(#[^)]+)?\)/g, "($1$2)");
  content = content.replace(/\(([^)#]+)\.md(#[^)]+)?\)/g, (m, p, hash) => {
    if (p.startsWith("http") || p.startsWith("/")) return m;
    return `(/ts/runtime/${p}${hash ?? ""})`;
  });

  content = content.replace(
    /^Defined in: \[([^\]]+)\]\(([^)]+)\)$/gm,
    "<!-- source: $1 -->\n[source]($2)"
  );

  content = content.replace(
    /```(?:ts|typescript)\n([^\n]+)\n```/g,
    (_, code) => `\`${code.replace(/;$/, "")}\``
  );

  if (file !== "index.md") {
    const lines = content.split("\n");
    const out = [];
    let openSymbol = null;
    for (const line of lines) {
      const h3 = line.match(/^### (.+)$/);
      const h2 = line.match(/^## .+$/);
      if (openSymbol && (h3 || h2)) {
        out.push("<!-- symbol-end -->");
        out.push("");
        openSymbol = null;
      }
      if (h3) {
        openSymbol = h3[1];
        out.push(`<!-- symbol-start: ${openSymbol} -->`);
      }
      out.push(line);
    }
    if (openSymbol) {
      out.push("");
      out.push("<!-- symbol-end -->");
    }
    content = out.join("\n");
  }

  const title = TITLES[file];
  if (title) {
    if (content.startsWith("---\n")) {
      const end = content.indexOf("\n---\n", 4);
      const head = content.slice(4, end);
      const rest = content.slice(end + 5);
      content = `---\ntitle: ${title}\n${head}\n---\n${rest}`;
    } else {
      content = `---\ntitle: ${title}\nlang: ts\ntoc: true\n---\n${content}`;
    }
    content = content.replace(/^# [^\n]+\n/m, `# ${title}\n`);
  }

  if (file === "index.md") {
    const labelMap = {
      "api/mod": "encore.dev/api",
      "auth/mod": "encore.dev/auth",
      "config/mod": "encore.dev/config",
      "cron/mod": "encore.dev/cron",
      "log/mod": "encore.dev/log",
      "metrics/mod": "encore.dev/metrics",
      "mod": "encore.dev",
      "pubsub/mod": "encore.dev/pubsub",
      "service/mod": "encore.dev/service",
      "storage/cache/mod": "encore.dev/storage/cache",
      "storage/objects/mod": "encore.dev/storage/objects",
      "storage/sqldb/mod": "encore.dev/storage/sqldb",
      "types/mod": "encore.dev/types",
      "validate/mod": "encore.dev/validate"
    };
    for (const [from, to] of Object.entries(labelMap)) {
      content = content.replaceAll(`[${from}]`, `[${to}]`);
    }
  }

  await writeFile(path, content);
}

console.log(`Post-processed ${finalFiles.length} files in ${dir.pathname}`);
