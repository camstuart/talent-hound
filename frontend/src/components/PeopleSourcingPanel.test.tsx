import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import PeopleSourcingPanel from "./PeopleSourcingPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, sourcingMocks, recordMocks, runtimeMocks } = vi.hoisted(() => {
  const state = {
    preview: null as Record<string, unknown> | null,
    leads: [] as Record<string, unknown>[],
    searches: [] as Record<string, unknown>[],
    previewError: "",
    sendError: "",
  };
  return {
    state,
    recordMocks: { ListRoles: vi.fn(async () => [{ id: 7, title: "Senior platform engineer" }]) },
    runtimeMocks: { Browser: { OpenURL: vi.fn(async () => undefined) } },
    sourcingMocks: {
      Preview: vi.fn(async () => {
        if (state.previewError) throw new Error(state.previewError);
        return state.preview;
      }),
      Inspect: vi.fn(async () => state.preview),
      Send: vi.fn(async () => {
        if (state.sendError) throw new Error(state.sendError);
        return { searchId: 1, leadIds: [1, 2], created: 2, alreadyInPool: 1, skipped: 0, partial: false };
      }),
      Leads: vi.fn(async () => state.leads),
      Searches: vi.fn(async () => state.searches),
      Suggest: vi.fn(async () => ({ fullName: "Quokka Stack", location: "", sourceNote: "Sourced from exa on 2026-08-26" })),
      Promote: vi.fn(async () => ({ id: 9, fullName: "Quokka Stack" })),
      Dismiss: vi.fn(async () => undefined),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  SourcingService: sourcingMocks,
  RecordService: recordMocks,
}));
vi.mock("@wailsio/runtime", () => runtimeMocks);

const aLead = (over: Record<string, unknown> = {}) => ({
  id: 1,
  url: "https://quokka.example.invalid/about",
  title: "Quokka Stack — platform engineer",
  snippet: "Builds local-first desktop tools.",
  state: "new",
  candidateId: null,
  candidateName: "",
  host: "quokka.example.invalid",
  ...over,
});

beforeEach(() => {
  state.preview = { query: "Go and production SQLite experience", organizationWarning: "", identifierWarning: "" };
  state.leads = [];
  state.searches = [];
  state.previewError = "";
  state.sendError = "";
  vi.clearAllMocks();
});

const chooseRole = async () => {
  const select = (await screen.findByLabelText("Search for role")) as HTMLSelectElement;
  await waitFor(() => expect(select.options.length).toBe(2));
  fireEvent.change(select, { target: { value: "7" } });
};

const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

describe("PeopleSourcingPanel", () => {
  it("builds a query for the chosen role and sends nothing by previewing", async () => {
    render(() => <PeopleSourcingPanel initiativeId={1} />);
    await chooseRole();
    await clickWhenReady("Build a people query");

    const field = (await screen.findByLabelText("People query to send")) as HTMLTextAreaElement;
    expect(field.value).toBe("Go and production SQLite experience");
    expect(sourcingMocks.Preview).toHaveBeenCalledWith(7);
    expect(sourcingMocks.Send).not.toHaveBeenCalled();
  });

  it("cancelling calls nothing and closes the editor", async () => {
    render(() => <PeopleSourcingPanel initiativeId={1} />);
    await chooseRole();
    await clickWhenReady("Build a people query");
    await screen.findByLabelText("People query to send");
    fireEvent.click(await screen.findByLabelText("Cancel this people search"));
    await waitFor(() => expect(screen.queryByLabelText("People query to send")).toBeNull());
    expect(sourcingMocks.Send).not.toHaveBeenCalled();
  });

  it("sends exactly what is in the box and reports the outcome", async () => {
    render(() => <PeopleSourcingPanel initiativeId={1} />);
    await chooseRole();
    await clickWhenReady("Build a people query");
    const field = (await screen.findByLabelText("People query to send")) as HTMLTextAreaElement;
    fireEvent.input(field, { target: { value: "Go engineers in Melbourne" } });
    await clickWhenReady("Send this people search");

    await waitFor(() => expect(sourcingMocks.Send).toHaveBeenCalledTimes(1));
    expect(sourcingMocks.Send.mock.calls[0][0]).toMatchObject({ initiativeId: 1, roleId: 7, query: "Go engineers in Melbourne" });
    const outcome = await screen.findByLabelText("People search outcome");
    expect(outcome.textContent).toContain("2 leads");
    expect(outcome.textContent).toContain("1 already in the pool");
  });

  it("lists leads with their state, and promotes one through the form", async () => {
    state.leads = [
      aLead(),
      aLead({ id: 2, url: "https://github.com/wombatdev", title: "wombatdev", host: "github.com", candidateId: 4, candidateName: "Wombat Developer" }),
      aLead({ id: 3, url: "https://example.invalid/gone", title: "Gone", state: "dismissed" }),
    ];
    render(() => <PeopleSourcingPanel initiativeId={1} />);
    const list = await screen.findByLabelText("Leads");
    await waitFor(() => expect(list.textContent).toContain("Quokka Stack"));
    expect(list.textContent).toContain("in pool as Wombat Developer");
    expect(list.textContent).toContain("(dismissed)");
    // Someone already in the pool cannot be promoted again.
    expect(screen.queryByLabelText("Promote wombatdev")).toBeNull();
    expect(screen.queryByLabelText("Promote Gone")).toBeNull();

    await clickWhenReady("Promote Quokka Stack — platform engineer");
    const form = await screen.findByLabelText("Promote to candidate");
    const name = form.querySelector('input[aria-label="Full name"]') as HTMLInputElement | null;
    await waitFor(() => expect((form.querySelector("input") as HTMLInputElement).value).toBe("Quokka Stack"));
    if (name) fireEvent.input(name, { target: { value: "Quokka Stack" } });
    fireEvent.submit(form);
    await waitFor(() => expect(sourcingMocks.Promote).toHaveBeenCalledTimes(1));
    expect(sourcingMocks.Promote.mock.calls[0][0]).toBe(1);
    expect(sourcingMocks.Promote.mock.calls[0][1]).toMatchObject({ fullName: "Quokka Stack" });
  });

  it("dismisses a lead and opens a page in the browser, never in the window", async () => {
    state.leads = [aLead()];
    render(() => <PeopleSourcingPanel initiativeId={1} />);
    await screen.findByLabelText("Lead Quokka Stack — platform engineer");
    await clickWhenReady("Open Quokka Stack — platform engineer");
    await waitFor(() => expect(runtimeMocks.Browser.OpenURL).toHaveBeenCalledWith("https://quokka.example.invalid/about"));
    expect(document.querySelector("iframe")).toBeNull();

    await clickWhenReady("Dismiss Quokka Stack — platform engineer");
    await waitFor(() => expect(sourcingMocks.Dismiss).toHaveBeenCalledWith(1));
  });

  it("shows the backend's own words when it refuses", async () => {
    state.previewError = "a query is built from a ready role profile: this role has not been profiled yet";
    render(() => <PeopleSourcingPanel initiativeId={1} />);
    await chooseRole();
    await clickWhenReady("Build a people query");
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("has not been profiled yet");
  });
});
