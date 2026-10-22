import { isTypingTarget } from "@/hooks/use-keyboard-shortcuts";

function el(html: string): HTMLElement {
  const host = document.createElement("div");
  host.innerHTML = html;
  document.body.append(host);
  return host.firstElementChild as HTMLElement;
}

describe("isTypingTarget", () => {
  it.each([
    ['<input type="text">', true],
    ['<input type="search">', true],
    ["<textarea></textarea>", true],
    ["<select></select>", true],
    ['<div contenteditable="true"></div>', true],
    ['<div class="xterm"><span></span></div>', true],
    ['<input type="checkbox">', false],
    ["<button>Go</button>", false],
    ["<a href='/'>Home</a>", false],
  ])("%s → %s", (html, expected) => {
    const node = el(html);
    const target = node.classList.contains("xterm") ? (node.firstElementChild as HTMLElement) : node;
    if (html.includes("contenteditable")) Object.defineProperty(target, "isContentEditable", { value: true });
    expect(isTypingTarget(target)).toBe(expected);
  });

  it("ignores non-elements", () => {
    expect(isTypingTarget(null)).toBe(false);
    expect(isTypingTarget(window)).toBe(false);
  });
});
