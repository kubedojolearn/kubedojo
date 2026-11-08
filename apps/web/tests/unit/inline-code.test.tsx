import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { InlineCode } from "@/components/lab/goals-panel";

describe("InlineCode", () => {
  it("renders backtick spans as code", () => {
    const { container } = render(<InlineCode text="Try `kubectl get pods` first." />);
    expect(container.textContent).toBe("Try kubectl get pods first.");
    expect(container.querySelector("code")?.textContent).toBe("kubectl get pods");
  });

  it("keeps an unmatched backtick as text", () => {
    const { container } = render(<InlineCode text="a `b" />);
    expect(container.textContent).toBe("a `b");
    expect(container.querySelector("code")).toBeNull();
  });
});
