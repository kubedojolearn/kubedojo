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

  it("only lists commands the simulator runs today as available", () => {
    render(<HomePage />);
    const today = screen.getByText("kubectl, today").closest("div")!;
    expect(today).not.toHaveTextContent("apply");
  });
});
