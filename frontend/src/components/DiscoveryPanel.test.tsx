import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import DiscoveryPanel from "./DiscoveryPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, discoveryMocks, recordMocks } = vi.hoisted(() => {
  const state = {
    preview: null as Record<string, unknown> | null,
    inspect: null as Record<string, unknown> | null,
    outcome: null as Record<string, unknown> | null,
    searches: [] as Record<string, unknown>[],
    previewError: "",
    sendError: "",
  };
  return {
    state,
    recordMocks: { ListCandidates: vi.fn(async () => [{ id: 1, fullName: "Kalinda Reyes" }]) },
    discoveryMocks: {
      Preview: vi.fn(async () => {
        if (state.previewError) throw new Error(state.previewError);
        return state.preview;
      }),
      Inspect: vi.fn(async () => state.inspect ?? state.preview),
      Send: vi.fn(async () => {
        if (state.sendError) throw new Error(state.sendError);
        return state.outcome ?? { searchId: 1, roleIds: [1], created: 1, updated: 0, skipped: 0, partial: false };
      }),
      Searches: vi.fn(async () => state.searches),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  DiscoveryService: discoveryMocks,
  RecordService: recordMocks,
}));

const aPreview = (over: Record<string, unknown> = {}) => ({
  query: "senior platform engineer, Go and SQLite in production",
  organizationWarning: "",
  identifierWarning: "",
  ...over,
});

beforeEach(() => {
  state.preview = aPreview();
  state.inspect = null;
  state.outcome = null;
  state.searches = [];
  state.previewError = "";
  state.sendError = "";
  vi.clearAllMocks();
});

// The panel disables its actions while a call is in flight, so a click waits
// for the previous one to settle rather than racing it.
const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

const buildQuery = async () => {
  const button = await screen.findByLabelText("Build a query");
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

describe("DiscoveryPanel", () => {
  it("shows the exact query that would be sent", async () => {
    render(() => <DiscoveryPanel initiativeId={1} />);
    await buildQuery();

    const field = (await screen.findByLabelText("Query to send")) as HTMLTextAreaElement;
    expect(field.value).toBe("senior platform engineer, Go and SQLite in production");
    // Nothing has been sent by previewing.
    expect(discoveryMocks.Send).not.toHaveBeenCalled();
  });

  it("sends exactly what is in the box, including edits", async () => {
    render(() => <DiscoveryPanel initiativeId={4} />);
    await buildQuery();

    const field = await screen.findByLabelText("Query to send");
    fireEvent.input(field, { target: { value: "platform engineer roles in Melbourne" } });
    await waitFor(() => expect(discoveryMocks.Inspect).toHaveBeenCalled());

    await clickWhenReady("Send this query");
    await waitFor(() =>
      expect(discoveryMocks.Send).toHaveBeenCalledWith({
        initiativeId: 4,
        candidateId: 0,
        query: "platform engineer roles in Melbourne",
        limit: 20,
      }),
    );
  });

  // Cancelling is the absence of the operation, not a recorded decision.
  it("cancels without calling anything", async () => {
    render(() => <DiscoveryPanel initiativeId={1} />);
    await buildQuery();
    await screen.findByLabelText("Query to send");

    fireEvent.click(screen.getByLabelText("Cancel this search"));
    await waitFor(() => expect(screen.queryByLabelText("Query to send")).toBeNull());
    expect(discoveryMocks.Send).not.toHaveBeenCalled();
  });

  it("warns about a named organization without blocking the send", async () => {
    state.preview = aPreview({
      query: "platform engineer roles at Northwind Pty Ltd",
      organizationWarning: "this query names Northwind Pty Ltd — searching for a specific organization is allowed",
    });
    render(() => <DiscoveryPanel initiativeId={1} />);
    await buildQuery();

    await screen.findByLabelText("Organization warning");
    expect(screen.queryByLabelText("Identifier warning")).toBeNull();
    // Still sendable: it is a deliberate human choice.
    expect(await screen.findByLabelText("Send this query")).toBeTruthy();
  });

  it("warns more strongly about a direct identifier", async () => {
    state.preview = aPreview({
      query: "platform engineer roles for Kalinda Reyes",
      identifierWarning: "this query contains what looks like a direct identifier (Kalinda Reyes)",
    });
    render(() => <DiscoveryPanel initiativeId={1} />);
    await buildQuery();

    const warning = await screen.findByLabelText("Identifier warning");
    // Shown as an error, not a note — the two are visually distinct.
    expect(warning.className).toContain("modal-error");
    expect(await screen.findByLabelText("Send this query")).toBeTruthy();
  });

  it("reports a partial result as partial", async () => {
    state.outcome = { searchId: 1, roleIds: [1, 2], created: 2, updated: 0, skipped: 1, partial: true };
    render(() => <DiscoveryPanel initiativeId={1} />);
    await buildQuery();
    await screen.findByLabelText("Query to send");
    await clickWhenReady("Send this query");

    const outcome = await screen.findByLabelText("Search outcome");
    expect(outcome.textContent).toContain("incomplete");
    expect(outcome.textContent).toContain("could not be read");
  });

  it("lists past searches with the query each one sent", async () => {
    state.searches = [
      {
        id: 1,
        initiativeId: 1,
        provider: "exa",
        query: "platform engineer roles in Melbourne",
        resultCount: 12,
        skippedCount: 0,
        partial: false,
        failureReason: "",
        sentAt: "2026-03-01T12:00:00Z",
      },
    ];
    render(() => <DiscoveryPanel initiativeId={1} />);

    const shown = await screen.findByLabelText("Query sent for search 1");
    expect(shown.tagName).toBe("PRE");
    expect(shown.textContent).toBe("platform engineer roles in Melbourne");
    expect(await screen.findByText(/12 results/)).toBeTruthy();
  });

  it("shows a failed search as its reason rather than as no results", async () => {
    state.searches = [
      {
        id: 1,
        initiativeId: 1,
        provider: "exa",
        query: "platform engineer",
        resultCount: 0,
        skippedCount: 0,
        partial: false,
        failureReason: "rate_limited",
        sentAt: "2026-03-01T12:00:00Z",
      },
    ];
    render(() => <DiscoveryPanel initiativeId={1} />);
    await screen.findByText(/rate limited/);
  });

  it("shows the backend's own words when there is nothing to search for", async () => {
    state.previewError = "there is nothing to search for yet — approve a profile or add criteria first";
    render(() => <DiscoveryPanel initiativeId={1} />);
    await buildQuery();
    await screen.findByText(/nothing to search for yet/);
  });
});
