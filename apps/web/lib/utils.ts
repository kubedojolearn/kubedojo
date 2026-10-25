import { createCn } from "cn/config";

/**
 * clsx + tailwind-merge, taught the type scale from globals.css. Without it, `text-small` looks
 * like a color and is dropped when merged with `text-foreground-secondary`.
 */
export const cn = createCn({
  extend: {
    classGroups: {
      "font-size": [{ text: ["display", "h1", "h2", "h3", "h4", "body-lg", "body", "small", "caption", "micro"] }],
    },
  },
});
