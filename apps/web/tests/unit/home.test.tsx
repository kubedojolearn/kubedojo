import { render, screen } from "@testing-library/react";
import Home from "@/app/page";

describe("home page", () => {
  it("names the product", () => {
    render(<Home />);
    expect(screen.getByRole("heading", { level: 1, name: "KubeDojo" })).toBeInTheDocument();
  });
});
