import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@solidjs/testing-library";
import RecordsPanel from "./RecordsPanel";

// The Go backend is not running: bindings are mocked. Fixtures are invented.
const { state, recordMocks } = vi.hoisted(() => {
  const state = {
    companies: [] as Record<string, unknown>[],
    // failNextListCompanies makes the next call to ListCompanies reject once,
    // which is what a dropped response looks like to the caller. Counting calls
    // instead would pin the number of reloads a create happens to issue rather
    // than what has to be true after one.
    failNextListCompanies: false,
    listCompaniesCalls: 0,
  };
  return {
    state,
    recordMocks: {
      ListCandidates: vi.fn(async () => []),
      ListRoles: vi.fn(async () => []),
      ListCompanies: vi.fn(async () => {
        state.listCompaniesCalls += 1;
        if (state.failNextListCompanies) {
          state.failNextListCompanies = false;
          throw new Error("the connection was reset");
        }
        // A fresh array each call, as the real bindings return: Solid compares
        // signal values by reference, so a reused array would never re-render.
        return [...state.companies];
      }),
      CreateCandidate: vi.fn(async () => ({ id: 1 })),
      CreateCompany: vi.fn(async (c: Record<string, unknown>) => {
        const row = { id: state.companies.length + 1, name: c.name };
        state.companies.push(row);
        return row;
      }),
      CreateRole: vi.fn(async () => ({ id: 1 })),
      CreateContact: vi.fn(async () => ({ id: 1 })),
      ContactsAtCompany: vi.fn(async () => ({ count: 0, contacts: [] })),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  RecordService: recordMocks,
}));

describe("RecordsPanel", () => {
  beforeEach(() => {
    state.companies = [];
    state.failNextListCompanies = false;
    state.listCompaniesCalls = 0;
    vi.clearAllMocks();
  });

  // Scoped to the Companies list: the name also appears as an option in the
  // contacts form's company selector, and an unscoped query matches both.
  const inCompanies = async (name: string) =>
    within(await screen.findByRole("region", { name: "Companies" })).findByText(name);

  const addCompany = async (name: string) => {
    const form = await screen.findByRole("form", { name: "New company" });
    fireEvent.input(within(form).getByLabelText(/^Name/), { target: { value: name } });
    fireEvent.click(within(form).getByRole("button", { name: "Add company" }));
  };

  it("shows a company it just created", async () => {
    render(() => <RecordsPanel />);
    await addCompany("Northwind Robotics");
    await inCompanies("Northwind Robotics");
  });

  // A created record that the database holds and the screen does not is the
  // worst of the three possible outcomes: the recruiter is told their entry
  // failed, it did not, and creating it again makes a duplicate.
  //
  // One create issues two reloads — the workspace bump and the awaited one —
  // and only the newest may write. So a newer reload that fails discards an
  // older one that succeeded, and writes nothing itself. The record is in the
  // database and the list still says it is not.
  it("still shows a created company when a refresh fails under it", async () => {
    render(() => <RecordsPanel />);
    await waitFor(() => expect(recordMocks.ListCompanies).toHaveBeenCalled());
    // The refresh the create triggers is the one that fails.
    state.failNextListCompanies = true;

    await addCompany("Northwind Robotics");

    await waitFor(
      async () => {
        await inCompanies("Northwind Robotics");
      },
      { timeout: 3000 },
    );
  });
});
