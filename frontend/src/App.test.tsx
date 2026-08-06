import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import App from "./App";

vi.mock("../bindings/camstuart/talent-hound", () => ({
  GreetService: {
    Greet: vi.fn(async (name: string) => `Hello ${name}, welcome to Talent Hound`),
  },
}));

describe("App", () => {
  it("shows the greeting returned by the (mocked) binding after typing a name and clicking Greet", async () => {
    render(() => <App />);

    const input = screen.getByPlaceholderText("Your name") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "Cam" } });

    fireEvent.click(screen.getByText("Greet"));

    await waitFor(() => {
      expect(screen.getByText("Hello Cam, welcome to Talent Hound")).toBeInTheDocument();
    });
  });
});
