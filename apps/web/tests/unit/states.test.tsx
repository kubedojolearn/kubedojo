import { render, screen } from "@testing-library/react";
import { EmptyState, ErrorState } from "@/components/feedback/states";

describe("feedback states", () => {
  it("EmptyState renders its heading at the requested level and no alert", () => {
    render(<EmptyState title="No lessons yet" description="Start one." headingLevel={3} />);
    expect(screen.getByRole("heading", { level: 3, name: "No lessons yet" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("ErrorState is announced as an alert", () => {
    render(<ErrorState title="Couldn't load" description="Try again." />);
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't load");
    expect(screen.getByRole("heading", { level: 2, name: "Couldn't load" })).toBeInTheDocument();
  });
});
