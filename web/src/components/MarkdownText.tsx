import { memo } from "react";
import { MarkdownTextPrimitive } from "@assistant-ui/react-markdown";
import "@assistant-ui/react-markdown/styles/dot.css";
import remarkGfm from "remark-gfm";

function MarkdownTextImpl() {
  return (
    <MarkdownTextPrimitive
      className="aui-md"
      remarkPlugins={[remarkGfm]}
      defer
    />
  );
}

export const MarkdownText = memo(MarkdownTextImpl);
