import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import CloudPanel from "./CloudPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, cloudMocks } = vi.hoisted(() => {
  const state = {
    endpoint: null as Record<string, unknown> | null,
    tasks: [] as Record<string, unknown>[],
    payload: null as Record<string, unknown> | null,
    configureError: "",
  };
  return {
    state,
    cloudMocks: {
      Endpoint: vi.fn(async () => state.endpoint),
      Tasks: vi.fn(async () => state.tasks),
      Configure: vi.fn(async () => {
        if (state.configureError) throw new Error(state.configureError);
        return { url: "https://api.example-cloud.invalid/v1", model: "m", revision: 2 };
      }),
      Remove: vi.fn(async () => undefined),
      Approve: vi.fn(async () => undefined),
      Revoke: vi.fn(async () => undefined),
      Preview: vi.fn(async () => state.payload),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({ CloudService: cloudMocks }));

const eligible = (task: string, over: Record<string, unknown> = {}) => ({
  task,
  denied: false,
  reason: "not approved for this initiative and endpoint",
  approved: false,
  ...over,
});

const denied = (task: string) => ({
  task,
  denied: true,
  reason: `${task} is local-only and cannot use a cloud endpoint under any configuration`,
  approved: false,
});

beforeEach(() => {
  state.endpoint = { url: "https://api.example-cloud.invalid/v1", model: "m", revision: 1 };
  state.tasks = [];
  state.payload = null;
  state.configureError = "";
  vi.clearAllMocks();
});

// The panel disables its actions while a load is in flight.
const clickWhenReady = async (label: string) => {
  const button = await screen.findByLabelText(label);
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(button);
};

describe("CloudPanel", () => {
  // A screen that shows only what is off invites someone to look for the switch
  // that turns on what is forbidden.
  it("lists the permanently denied tasks alongside the approvable ones", async () => {
    state.tasks = [eligible("drafting"), denied("embedding")];
    render(() => <CloudPanel initiativeId={1} />);

    await screen.findByText(/Writing drafts/);
    await screen.findByText(/Embedding evidence/);
    // The denied one says "never" and offers no approval.
    await screen.findByText(/— never/);
    expect(screen.queryByLabelText("Approve embedding")).toBeNull();
    expect(screen.queryByLabelText("Preview the payload for embedding")).toBeNull();
  });

  it("shows why a denied task can never be enabled", async () => {
    state.tasks = [denied("candidate_extraction")];
    render(() => <CloudPanel initiativeId={1} />);

    const reason = await screen.findByLabelText("Why candidate_extraction is not in use");
    expect(reason.textContent).toContain("local-only");
    expect(reason.textContent).toContain("any configuration");
    // Shown as a rule, not as a pending state.
    expect(reason.className).toContain("modal-error");
  });

  it("approves and revokes a task for this initiative", async () => {
    state.tasks = [eligible("drafting")];
    render(() => <CloudPanel initiativeId={5} />);

    fireEvent.click(await screen.findByLabelText("Approve drafting"));
    await waitFor(() => expect(cloudMocks.Approve).toHaveBeenCalledWith(5, "drafting"));

    state.tasks = [eligible("drafting", { approved: true, reason: "" })];
    render(() => <CloudPanel initiativeId={5} />);
    fireEvent.click(await screen.findByLabelText("Revoke drafting"));
    await waitFor(() => expect(cloudMocks.Revoke).toHaveBeenCalledWith(5, "drafting"));
  });

  it("shows the exact payload rather than a description of it", async () => {
    state.tasks = [eligible("drafting")];
    state.payload = {
      task: "drafting",
      text: "[candidate name] has five years of production Go.",
      endpoint: "https://api.example-cloud.invalid/v1",
      model: "m",
    };
    render(() => <CloudPanel initiativeId={1} />);

    fireEvent.click(await screen.findByLabelText("Preview the payload for drafting"));
    const shown = await screen.findByLabelText("Payload text");
    expect(shown.tagName).toBe("PRE");
    // Identifiers are already replaced, because substitution happens before the
    // preview.
    expect(shown.textContent).toContain("[candidate name]");
  });

  it("says that changing the endpoint clears every approval", async () => {
    render(() => <CloudPanel initiativeId={1} />);
    const state1 = await screen.findByLabelText("Cloud endpoint state");
    expect(state1.textContent).toContain("revision 1");
    expect(state1.textContent).toContain("clears every approval");
  });

  it("configures and removes the endpoint", async () => {
    render(() => <CloudPanel initiativeId={1} />);
    const url = await screen.findByLabelText("Cloud endpoint URL");
    fireEvent.input(url, { target: { value: "https://api.another-cloud.invalid/v1" } });
    await clickWhenReady("Save the cloud endpoint");
    await waitFor(() =>
      expect(cloudMocks.Configure).toHaveBeenCalledWith("https://api.another-cloud.invalid/v1", "m"),
    );

    fireEvent.click(await screen.findByLabelText("Remove the cloud endpoint"));
    await waitFor(() => expect(cloudMocks.Remove).toHaveBeenCalled());
  });

  it("shows the backend's own words when configuring is refused", async () => {
    state.configureError = "the cloud endpoint must be an absolute http or https URL";
    render(() => <CloudPanel initiativeId={1} />);
    await clickWhenReady("Save the cloud endpoint");
    await screen.findByText(/must be an absolute http or https URL/);
  });

  it("offers no endpoint controls beyond the one", async () => {
    state.endpoint = null;
    render(() => <CloudPanel initiativeId={1} />);
    await screen.findByLabelText("Cloud endpoint URL");
    // With nothing configured there is nothing to remove.
    expect(screen.queryByLabelText("Remove the cloud endpoint")).toBeNull();
  });
});
