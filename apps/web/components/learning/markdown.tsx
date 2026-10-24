import { isValidElement, type ReactNode } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { CopyButton } from "@/components/learning/copy-button";
import { cn } from "@/lib/utils";

function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (isValidElement<{ children?: ReactNode }>(node)) return textOf(node.props.children);
  return "";
}

const components: Components = {
  h1: ({ children }) => <h2 className="mt-10 text-2xl font-semibold tracking-tight">{children}</h2>,
  h2: ({ children }) => <h2 className="mt-10 text-xl font-semibold tracking-tight">{children}</h2>,
  h3: ({ children }) => <h3 className="mt-8 text-lg font-semibold">{children}</h3>,
  p: ({ children }) => <p className="mt-4 leading-7">{children}</p>,
  ul: ({ children }) => <ul className="mt-4 list-disc space-y-2 pl-6">{children}</ul>,
  ol: ({ children }) => <ol className="mt-4 list-decimal space-y-2 pl-6">{children}</ol>,
  a: ({ href, children }) => {
    const external = href?.startsWith("http");
    return (
      <a href={href} className="text-link underline underline-offset-4" {...(external ? { rel: "noreferrer" } : {})}>
        {children}
      </a>
    );
  },
  blockquote: ({ children }) => (
    <aside className="mt-6 rounded-lg border-l-4 border-warning bg-sim-badge/40 px-4 py-1 text-sm">{children}</aside>
  ),
  table: ({ children }) => (
    <div className="mt-6 overflow-x-auto rounded-lg border">
      <table className="w-full text-left text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => <th className="border-b bg-muted px-3 py-2 font-semibold">{children}</th>,
  td: ({ children }) => <td className="border-b px-3 py-2 align-top">{children}</td>,
  code: ({ className, children }) => {
    // Fenced blocks are rendered by `pre`; this handles inline code.
    if (className?.startsWith("language-")) return <code className={className}>{children}</code>;
    return <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-[0.9em]">{children}</code>;
  },
  pre: ({ children }) => {
    const text = textOf(children).replace(/\n$/, "");
    return (
      <div className="relative mt-4">
        <pre className="overflow-x-auto rounded-lg bg-terminal p-4 pr-12 font-mono text-sm text-terminal-foreground">
          {children}
        </pre>
        <div className="absolute top-2 right-2">
          <CopyButton text={text} />
        </div>
      </div>
    );
  },
};

/**
 * Renders reviewed lesson Markdown (GitHub-flavoured). Raw HTML is dropped, never rendered, and
 * no MDX or scripts are evaluated (ADR-0005, threat T11).
 */
export function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div className={cn("max-w-3xl text-pretty", className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}
