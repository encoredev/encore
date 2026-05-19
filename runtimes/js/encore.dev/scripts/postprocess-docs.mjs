import { readdir, readFile, writeFile, unlink, rename } from "node:fs/promises";
import { join } from "node:path";

const DOCS_DIR = new URL("../../../../docs/ts/runtime/", import.meta.url);

const RENAMES = {
  "mod.mdx": "encore-dev.mdx",
  "api.mod.mdx": "api.mdx",
  "auth.mod.mdx": "auth.mdx",
  "config.mod.mdx": "config.mdx",
  "cron.mod.mdx": "cron.mdx",
  "log.mod.mdx": "log.mdx",
  "metrics.mod.mdx": "metrics.mdx",
  "pubsub.mod.mdx": "pubsub.mdx",
  "service.mod.mdx": "service.mdx",
  "storage.cache.mod.mdx": "storage-cache.mdx",
  "storage.objects.mod.mdx": "storage-objects.mdx",
  "storage.sqldb.mod.mdx": "storage-sqldb.mdx",
  "types.mod.mdx": "types.mdx",
  "validate.mod.mdx": "validate.mdx"
};

const TITLES = {
  "encore-dev.mdx": "encore.dev",
  "api.mdx": "encore.dev/api",
  "auth.mdx": "encore.dev/auth",
  "config.mdx": "encore.dev/config",
  "cron.mdx": "encore.dev/cron",
  "log.mdx": "encore.dev/log",
  "metrics.mdx": "encore.dev/metrics",
  "pubsub.mdx": "encore.dev/pubsub",
  "service.mdx": "encore.dev/service",
  "storage-cache.mdx": "encore.dev/storage/cache",
  "storage-objects.mdx": "encore.dev/storage/objects",
  "storage-sqldb.mdx": "encore.dev/storage/sqldb",
  "types.mdx": "encore.dev/types",
  "validate.mdx": "encore.dev/validate"
};

const dir = new URL(DOCS_DIR);

const files = await readdir(dir);

if (files.includes("index.mdx")) {
  await unlink(new URL("index.mdx", dir));
}

for (const [from, to] of Object.entries(RENAMES)) {
  if (files.includes(from)) {
    await rename(new URL(from, dir), new URL(to, dir));
  }
}

const finalFiles = await readdir(dir);
for (const file of finalFiles) {
  if (!file.endsWith(".mdx")) continue;
  const path = new URL(file, dir);
  let content = await readFile(path, "utf8");

  for (const [from, to] of Object.entries(RENAMES)) {
    const fromBase = from.replace(/\.mdx$/, "");
    const toBase = to.replace(/\.mdx$/, "");
    content = content.replaceAll(`/ts/runtime/${from}`, `/ts/runtime/${toBase}`);
    content = content.replaceAll(`/ts/runtime/${fromBase}`, `/ts/runtime/${toBase}`);
    content = content.replaceAll(`(${from})`, `(/ts/runtime/${toBase})`);
    content = content.replaceAll(`(./${from})`, `(/ts/runtime/${toBase})`);
  }

  content = content.replaceAll("(index.mdx)", "(/ts/runtime/)");
  content = content.replaceAll("(./index.mdx)", "(/ts/runtime/)");
  content = content.replace(/\((\/ts\/runtime\/[^)#]+)\.mdx(#[^)]+)?\)/g, "($1$2)");
  content = content.replace(/\(([^)#]+)\.mdx(#[^)]+)?\)/g, (m, p, hash) => {
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

  {
    const lines = content.split("\n");
    const filtered = [];
    let headingLevel = 0;
    let headingText = "";
    let lastHeadingIndex = -1;
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];
      const headingMatch = line.match(/^(#+) (.+)$/);
      if (headingMatch) {
        headingLevel = headingMatch[1].length;
        headingText = headingMatch[2];
        lastHeadingIndex = filtered.length;
      }
      if (
        line.startsWith("<!-- source:") &&
        (lines[i + 1] ?? "").startsWith("[source](")
      ) {
        const keep =
          headingLevel <= 3 ||
          /\(\)$/.test(headingText) ||
          headingText === "Constructor";
        if (keep && lastHeadingIndex >= 0) {
          const url = lines[i + 1].match(/\[source\]\(([^)]+)\)/)?.[1];
          if (url) {
            filtered[lastHeadingIndex] += ` <SymbolSource href="${url}" />`;
          }
        }
        i += 2;
        if (lines[i] === "") i++;
        continue;
      }
      filtered.push(line);
      i++;
    }
    content = filtered.join("\n");
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
  }

  if (file === "index.mdx") {
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
