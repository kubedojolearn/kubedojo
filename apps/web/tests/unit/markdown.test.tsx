import { render, screen } from "@testing-library/react";
import { Markdown } from "@/components/learning/markdown";

describe("Markdown", () => {
  it("renders GitHub-flavoured Markdown", () => {
    render(
      <Markdown>
        {"## Title\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\n`kubectl` and [docs](https://kubernetes.io)"}
      </Markdown>,
    );
    expect(screen.getByRole("heading", { level: 2, name: "Title" })).toBeInTheDocument();
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute("rel", "noreferrer");
  });

  it("never renders raw HTML or scripts (stored XSS)", () => {
    const { container } = render(
      <Markdown>
        {
          'Hello <img src=x onerror="alert(1)"> <script>alert(2)</script> <a href="javascript:alert(3)">x</a>\n\n[click](javascript:alert(4))'
        }
      </Markdown>,
    );
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
    for (const a of container.querySelectorAll("a")) expect(a.getAttribute("href") ?? "").not.toMatch(/^javascript:/i);
  });

  it("adds a copy button to code blocks", () => {
    render(<Markdown>{"```sh\nkubectl get pods\n```"}</Markdown>);
    expect(screen.getByRole("button", { name: "Copy code" })).toBeInTheDocument();
    expect(screen.getByText("kubectl get pods")).toBeInTheDocument();
  });
});
