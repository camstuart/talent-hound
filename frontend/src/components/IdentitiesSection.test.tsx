import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import IdentitiesSection from "./IdentitiesSection";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, enrichMocks, runtimeMocks } = vi.hoisted(() => {
  const state = {
    identities: [] as Record<string, unknown>[],
    preview: { handles: [] as string[], endpoints: ["/users/{login}"], tokenStored: false, reason: "" },
    addError: "",
    runError: "",
  };
  return {
    state,
    runtimeMocks: { Browser: { OpenURL: vi.fn(async () => undefined) } },
    enrichMocks: {
      Identities: vi.fn(async () => state.identities),
      Preview: vi.fn(async () => state.preview),
      AddIdentity: vi.fn(async () => {
        if (state.addError) throw new Error(state.addError);
        return { id: 3 };
      }),
      RemoveIdentity: vi.fn(async () => undefined),
      Run: vi.fn(async () => {
        if (state.runError) throw new Error(state.runError);
        return { artifactIds: [10, 11, 12], unchanged: 0, partial: false, failureReason: "" };
      }),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({ EnrichService: enrichMocks }));
vi.mock("@wailsio/runtime", () => runtimeMocks);

const github = { id: 1, candidateId: 5, provider: "github", handle: "wombatdev", url: "https://github.com/wombatdev", verifiedAt: "" };

beforeEach(() => {
  state.identities = [];
  state.preview = { handles: [], endpoints: ["/users/{login}"], tokenStored: false, reason: "this candidate has no GitHub identity to read" };
  state.addError = "";
  state.runError = "";
  vi.clearAllMocks();
});

const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

describe("IdentitiesSection", () => {
  it("lists identities and adds one from a URL", async () => {
    state.identities = [{ ...github, verifiedAt: "2026-08-26" }];
    render(() => <IdentitiesSection candidateId={5} />);
    const list = await screen.findByLabelText("Identity list");
    await waitFor(() => expect(list.textContent).toContain("github: wombatdev"));
    expect(list.textContent).toContain("confirmed 2026-08-26");

    fireEvent.input(screen.getByLabelText("Identity URL"), { target: { value: "https://wombat.example.invalid" } });
    await clickWhenReady("Add this identity");
    await waitFor(() => expect(enrichMocks.AddIdentity).toHaveBeenCalledWith(5, "", "https://wombat.example.invalid"));
  });

  it("removes an identity and opens one in the browser", async () => {
    state.identities = [github];
    render(() => <IdentitiesSection candidateId={5} />);
    await screen.findByLabelText("Identity github wombatdev");
    await clickWhenReady("Open github wombatdev");
    await waitFor(() => expect(runtimeMocks.Browser.OpenURL).toHaveBeenCalledWith("https://github.com/wombatdev"));
    await clickWhenReady("Remove github wombatdev");
    await waitFor(() => expect(enrichMocks.RemoveIdentity).toHaveBeenCalledWith(1));
  });

  it("disables enrichment with the backend's reason", async () => {
    render(() => <IdentitiesSection candidateId={5} />);
    const button = (await screen.findByLabelText("Enrich from GitHub")) as HTMLButtonElement;
    await waitFor(() => expect(button.disabled).toBe(true));
    expect((await screen.findByLabelText("Enrich unavailable reason")).textContent).toContain("no GitHub identity");
    expect(enrichMocks.Run).not.toHaveBeenCalled();
  });

  it("previews exactly what would be sent, then runs on confirmation", async () => {
    state.identities = [github];
    state.preview = { handles: ["wombatdev"], endpoints: ["/users/{login}", "/users/{login}/repos"], tokenStored: true, reason: "" };
    render(() => <IdentitiesSection candidateId={5} />);
    await clickWhenReady("Enrich from GitHub");
    const region = await screen.findByLabelText("Enrichment preview");
    expect(region.textContent).toContain("wombatdev");
    expect(region.textContent).toContain("/users/{login}/repos");
    expect(enrichMocks.Run).not.toHaveBeenCalled();

    fireEvent.click(screen.getByLabelText("Cancel enrichment"));
    await waitFor(() => expect(screen.queryByLabelText("Enrichment preview")).toBeNull());
    expect(enrichMocks.Run).not.toHaveBeenCalled();

    await clickWhenReady("Enrich from GitHub");
    await clickWhenReady("Run enrichment");
    await waitFor(() => expect(enrichMocks.Run).toHaveBeenCalledWith(5));
    expect((await screen.findByLabelText("Enrichment outcome")).textContent).toContain("3 artifacts added");
  });

  it("shows the backend's own words when it refuses", async () => {
    state.addError = "a GitHub identity is a profile URL like https://github.com/login";
    render(() => <IdentitiesSection candidateId={5} />);
    fireEvent.input(await screen.findByLabelText("Identity URL"), { target: { value: "https://x.invalid" } });
    await clickWhenReady("Add this identity");
    expect((await screen.findByRole("alert")).textContent).toContain("profile URL");
  });
});
