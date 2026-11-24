import { render, screen } from "@testing-library/react";
import HomePage from "@/app/(marketing)/page";

describe("home page", () => {
  it("has one h1 with the tagline", () => {
    render(<HomePage />);
    const headings = screen.getAllByRole("heading", { level: 1 });
    expect(headings).toHaveLength(1);
    expect(headings[0]).toHaveTextContent("Master Kubernetes, one command at a time.");
  });

  it("labels the terminal example as simulated", () => {
    render(<HomePage />);
    expect(screen.getByText(/The cluster is simulated: nothing runs/)).toBeInTheDocument();
  });

  it("lists all six courses as belts", () => {
    render(<HomePage />);
    const curriculum = screen.getByRole("heading", { name: /^Six belts/ }).closest("section")!;
    expect(curriculum.querySelectorAll("li")).toHaveLength(6);
    expect(curriculum).toHaveTextContent("white belt");
    expect(curriculum).toHaveTextContent("black belt");
  });

  it("lists read and change commands separately", () => {
    render(<HomePage />);
    const read = screen.getByText("kubectl: look around").closest("div")!;
    const change = screen.getByText("kubectl: change things").closest("div")!;
    expect(read).toHaveTextContent("describe");
    expect(read).not.toHaveTextContent("apply");
    expect(change).toHaveTextContent("apply -f");
    // `edit` is not supported by the simulator, so it must not be advertised.
    const chips = Array.from(change.querySelectorAll("code"), (c) => c.textContent);
    expect(chips).not.toContain("edit");
  });
});
