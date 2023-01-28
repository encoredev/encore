import { ClipboardDocumentCheckIcon, ClipboardDocumentIcon } from "@heroicons/react/24/outline";
import hljs from "highlight.js";
import Prism from "prismjs";
import "prismjs/components/prism-cue";
import "prismjs/components/prism-protobuf";
import React, { FC, PropsWithChildren, useEffect, useState } from "react";
import { copyToClipboard } from "~lib/clipboard";

type Language = "go" | "bash" | "js" | "ts" | "css" | "cue" | "protobuf" | "output" | "sql";

function isPrismLanguage(lang: Language | null): boolean {
  return lang === "cue" || lang === "protobuf";
}

const Code: FC<PropsWithChildren<{ lang: Language; rawContents: string }>> = ({
  lang,
  rawContents,
}) => {
  let code: React.ReactNode = rawContents;

  useEffect(() => {
    hljs.highlightAll();
  });

  if (isPrismLanguage(lang)) {
    const html = Prism.highlight(rawContents, Prism.languages.cue, "cue");
    code = <span dangerouslySetInnerHTML={{ __html: html }} />;
  }

  return (
    <div className="relative h-full text-xs">
      <pre className="h-full">
        <code
          className={
            lang
              ? (isPrismLanguage(lang) ? "prismjs" : "hljs") + ` h-full !p-3 language-${lang}`
              : ""
          }
        >
          {code}
        </code>
      </pre>
      {lang !== "output" && (
        <CopyButton
          className="absolute top-[0.5em] right-2 h-6 text-white"
          contents={rawContents}
        />
      )}
    </div>
  );
};

export default Code;

export const CopyButton: FC<{ contents: string; className?: string }> = ({
  contents,
  className,
}) => {
  const [hasCopied, setHasCopied] = useState(false);
  const onClick = () => {
    copyToClipboard(contents);

    setHasCopied(true);
    setTimeout(() => {
      setHasCopied(false);
    }, 1000);
  };

  return (
    <button
      className={`inline-flex cursor-pointer items-center mobile:hidden ${className ?? ""}`}
      title="Copy"
      onClick={onClick}
    >
      {hasCopied ? (
        <ClipboardDocumentCheckIcon className="h-6 w-6" />
      ) : (
        <ClipboardDocumentIcon className="h-6 w-6 hover:opacity-70" />
      )}
    </button>
  );
};
