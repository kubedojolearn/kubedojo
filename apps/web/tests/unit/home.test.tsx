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
    expect(screen.getByText(/The cluster is simulated: no containers run/)).toBeInTheDocument();
  });

  it("lists all six courses", () => {
    render(<HomePage />);
    const curriculum = screen.getByRole("heading", { name: "What you'll learn" }).closest("section")!;
    expect(curriculum.querySelectorAll("li")).toHaveLength(6);
  });
});
