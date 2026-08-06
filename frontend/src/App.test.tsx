import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import App from "./App";

const { mockCreate, mockList } = vi.hoisted(() => {
  let nextId = 1;
  return {
    mockCreate: vi.fn(async (name: string, type: string) => ({
      id: nextId++,
      name,
      type,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    })),
    mockList: vi.fn(async (): Promise<unknown[]> => []),
  };
});

vi.mock("../bindings/camstuart/talent-hound", () => ({
  InitiativeService: {
    Create: mockCreate,
    List: mockList,
  },
}));

beforeEach(() => {
  mockCreate.mockClear();
  mockList.mockClear();
});

async function createInitiative(name: string, type: string) {
  fireEvent.click(screen.getByLabelText("New initiative"));
  const dialog = await screen.findByRole("dialog", { name: "New initiative" });
  fireEvent.input(screen.getByPlaceholderText("Initiative name"), { target: { value: name } });
  fireEvent.change(screen.getByLabelText("Initiative type"), { target: { value: type } });
  fireEvent.click(screen.getByText("Create"));
  await waitFor(() => expect(dialog).not.toBeInTheDocument());
}

describe("App", () => {
  it("loads existing initiatives into the sidebar on mount", async () => {
    mockList.mockResolvedValueOnce([
      { id: 7, name: "Existing hunt", type: "talent_search", createdAt: "", updatedAt: "" },
    ]);
    render(() => <App />);

    await waitFor(() => {
      expect(screen.getByText("Existing hunt")).toBeInTheDocument();
    });
    // Listed but not opened: no tab yet.
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  });

  it("creates an initiative via the modal, adds it to the sidebar and opens a tab", async () => {
    render(() => <App />);

    await createInitiative("Hire Go devs", "talent_search");

    expect(mockCreate).toHaveBeenCalledWith("Hire Go devs", "talent_search");

    const tab = screen.getByRole("tab", { name: /Hire Go devs/ });
    expect(tab).toBeInTheDocument();
    // Sidebar entry, tab title and panel header all render the name.
    expect(screen.getAllByText("Hire Go devs")).toHaveLength(3);
    // The type icon renders in both sidebar and tab.
    expect(document.querySelectorAll('[data-icon="talent_search"]').length).toBeGreaterThanOrEqual(2);
  });

  it("closes a tab with its close button, keeping the initiative in the sidebar", async () => {
    render(() => <App />);

    await createInitiative("BD pipeline", "business_development");
    expect(screen.getByRole("tab", { name: /BD pipeline/ })).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Close BD pipeline"));

    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    // Still listed in the sidebar; welcome screen is back.
    expect(screen.getByText("BD pipeline")).toBeInTheDocument();
    expect(screen.getByText(/Create an initiative from the panel/)).toBeInTheDocument();
  });

  it("activates the neighbouring tab when the active tab is closed", async () => {
    render(() => <App />);

    await createInitiative("First", "job_search");
    await createInitiative("Second", "talent_search");

    expect(screen.getByRole("tab", { name: /Second/ })).toHaveAttribute("aria-selected", "true");

    fireEvent.click(screen.getByLabelText("Close Second"));

    expect(screen.getByRole("tab", { name: /First/ })).toHaveAttribute("aria-selected", "true");
  });
});
