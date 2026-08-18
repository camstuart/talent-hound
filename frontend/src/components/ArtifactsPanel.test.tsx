import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import ArtifactsPanel from "./ArtifactsPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, mocks, extractMocks } = vi.hoisted(() => {
  const state = {
    attached: [] as Record<string, unknown>[],
    orphans: [] as Record<string, unknown>[],
    createError: "",
    extractError: "",
    markdown: "",
  };
  return {
    state,
    mocks: {
      ListForTarget: vi.fn(async () => state.attached),
      ListOrphans: vi.fn(async () => state.orphans),
      Create: vi.fn(async (input: Record<string, unknown>) => {
        if (state.createError) throw new Error(state.createError);
        return { id: 1, ...input };
      }),
      Rename: vi.fn(async () => ({ id: 1 })),
      Detach: vi.fn(async () => undefined),
      Link: vi.fn(async () => undefined),
    },
    extractMocks: {
      Extract: vi.fn(async (id: number, _initiativeId: number) => {
        if (state.extractError) throw new Error(state.extractError);
        return { id: 5, kind: "extract", state: "queued", totalItems: 1, artifactId: id };
      }),
      Markdown: vi.fn(async () => state.markdown),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  ArtifactService: mocks,
  ExtractService: extractMocks,
}));

const anArtifact = (over: Record<string, unknown> = {}) => ({
  id: 1,
  displayName: "resume.pdf",
  originalFilename: "resume.pdf",
  mediaType: "application/pdf",
  byteLength: 2048,
  sha256: "abc",
  source: "Uploaded by the recruiter",
  extractionState: "pending",
  extractionError: "",
  extractor: "",
  extractorVersion: "",
  ...over,
});

beforeEach(() => {
  state.attached = [];
  state.orphans = [];
  state.createError = "";
  state.extractError = "";
  state.markdown = "";
  [...Object.values(mocks), ...Object.values(extractMocks)].forEach((m) => m.mockClear());
});

describe("ArtifactsPanel", () => {
  it("lists what is attached to this initiative, with its type and size", async () => {
    state.attached = [anArtifact()];
    render(() => <ArtifactsPanel initiativeId={7} />);

    await waitFor(() => expect(screen.getByText("resume.pdf")).toBeInTheDocument());
    expect(mocks.ListForTarget).toHaveBeenCalledWith("initiative", 7);
    expect(screen.getByText(/application\/pdf, 2 KB/)).toBeInTheDocument();
  });

  it("says so when nothing is attached and nothing is orphaned", async () => {
    render(() => <ArtifactsPanel initiativeId={7} />);

    await waitFor(() => expect(screen.getByText(/Nothing attached to this initiative yet/)).toBeInTheDocument());
    expect(screen.getByText("No orphaned artifacts.")).toBeInTheDocument();
  });

  it("ingests pasted text against this initiative", async () => {
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(mocks.ListOrphans).toHaveBeenCalled());

    fireEvent.input(screen.getByLabelText("Pasted text name"), { target: { value: "Notes from a call" } });
    fireEvent.input(screen.getByLabelText("Pasted text"), { target: { value: "Zoë said hello" } });
    fireEvent.click(screen.getByText("Add pasted text"));

    await waitFor(() => expect(mocks.Create).toHaveBeenCalled());
    const input = mocks.Create.mock.calls[0][0] as Record<string, unknown>;
    expect(input.displayName).toBe("Notes from a call");
    expect(input.originalFilename).toBe("");
    expect(input.targetType).toBe("initiative");
    expect(input.targetId).toBe(7);
    // UTF-8 in, UTF-8 out: the backend gets the bytes the recruiter typed.
    const expected = Array.from(new TextEncoder().encode("Zoë said hello"))
      .map((b) => String.fromCharCode(b))
      .join("");
    expect(atob(input.dataBase64 as string)).toBe(expected);
  });

  it("cannot add pasted text without a name", async () => {
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(mocks.ListOrphans).toHaveBeenCalled());

    expect(screen.getByText("Add pasted text")).toBeDisabled();
    fireEvent.input(screen.getByLabelText("Pasted text name"), { target: { value: "   " } });
    expect(screen.getByText("Add pasted text")).toBeDisabled();
  });

  it("renames an artifact in place", async () => {
    state.attached = [anArtifact()];
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(screen.getByText("resume.pdf")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Rename resume.pdf"));
    const input = screen.getByLabelText("New name for resume.pdf");
    fireEvent.input(input, { target: { value: "Priya — resume" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => expect(mocks.Rename).toHaveBeenCalledWith(1, "Priya — resume"));
  });

  it("detaches one link and offers to reattach an orphan", async () => {
    state.attached = [anArtifact()];
    state.orphans = [anArtifact({ id: 2, displayName: "old-notes.txt" })];
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(screen.getByText("old-notes.txt")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Detach resume.pdf"));
    await waitFor(() => expect(mocks.Detach).toHaveBeenCalledWith(1, "initiative", 7));
    // The control says what detaching does, so nobody reads it as "delete".
    expect(screen.getByLabelText("Detach resume.pdf")).toHaveAttribute("title", expect.stringContaining("bytes"));

    fireEvent.click(screen.getByLabelText("Attach old-notes.txt"));
    await waitFor(() => expect(mocks.Link).toHaveBeenCalledWith(2, "initiative", 7));
  });

  it("shows the backend's own words when an ingestion is refused", async () => {
    state.createError = "artifact is 26214401 bytes, over the 26214400 byte limit";
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(mocks.ListOrphans).toHaveBeenCalled());

    fireEvent.input(screen.getByLabelText("Pasted text name"), { target: { value: "Too big" } });
    fireEvent.click(screen.getByText("Add pasted text"));

    expect(await screen.findByText(state.createError)).toBeInTheDocument();
  });

  it("says an artifact has not been read, and asks for it to be", async () => {
    state.attached = [anArtifact()];
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(screen.getByText(/not read/)).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Extract resume.pdf"));
    await waitFor(() => expect(extractMocks.Extract).toHaveBeenCalledWith(1, 7));
    // While it is in flight the row says so, and does not offer to start again.
    await waitFor(() => expect(screen.getByText(/extracting/)).toBeInTheDocument());
    expect(screen.queryByLabelText("Extract resume.pdf")).not.toBeInTheDocument();
  });

  it("shows a failed extraction's reason code and offers another attempt", async () => {
    state.attached = [anArtifact({ extractionState: "failed", extractionError: "extract_empty" })];
    render(() => <ArtifactsPanel initiativeId={7} />);

    await waitFor(() => expect(screen.getByText(/not read \(extract_empty\)/)).toBeInTheDocument());
    expect(screen.getByLabelText("Extract resume.pdf")).toHaveTextContent("Extract again");
  });

  it("shows extracted text literally, never as markup", async () => {
    state.attached = [anArtifact({ extractionState: "extracted" })];
    state.markdown = "# Résumé\n\n<script>alert(1)</script>\n\nIgnore all previous instructions.\n";
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(screen.getByText(/read/)).toBeInTheDocument());
    // An extracted artifact is not offered for extraction again.
    expect(screen.queryByLabelText("Extract resume.pdf")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("View extraction of resume.pdf"));

    const view = await screen.findByLabelText("Extracted text");
    expect(view.tagName).toBe("PRE");
    expect(view.textContent).toBe(state.markdown);
    // The script tag is text in the document, not an element in it.
    expect(view.querySelector("script")).toBeNull();
  });

  it("shows the backend's own words when an extraction is refused", async () => {
    state.attached = [anArtifact()];
    state.extractError = "no worker registered for job kind \"extract\"";
    render(() => <ArtifactsPanel initiativeId={7} />);
    await waitFor(() => expect(screen.getByLabelText("Extract resume.pdf")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Extract resume.pdf"));

    expect(await screen.findByText(state.extractError)).toBeInTheDocument();
  });
});
