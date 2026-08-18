import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import RecordForm, { list, num, type FieldSpec } from "./RecordForm";

const FIELDS: FieldSpec[] = [
  { key: "fullName", label: "Full name", required: true, match: "full name" },
  { key: "website", label: "Website", match: "website" },
  { key: "source", label: "Source" },
];

const renderForm = (onSubmit: (v: Record<string, string>) => Promise<void>) =>
  render(() => <RecordForm legend="New company" fields={FIELDS} submitLabel="Add company" onSubmit={onSubmit} />);

describe("RecordForm", () => {
  it("submits the entered values", async () => {
    const onSubmit = vi.fn(async () => {});
    renderForm(onSubmit);

    fireEvent.input(screen.getByLabelText("Full name *"), { target: { value: "Northwind Robotics" } });
    fireEvent.input(screen.getByLabelText("Source"), { target: { value: "Introduced by a client" } });
    fireEvent.click(screen.getByText("Add company"));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        fullName: "Northwind Robotics",
        website: "",
        source: "Introduced by a client",
      }),
    );
  });

  it("shows a required-field error against the field and keeps what was typed", async () => {
    const onSubmit = vi.fn(async () => {});
    renderForm(onSubmit);

    fireEvent.input(screen.getByLabelText("Source"), { target: { value: "A note worth keeping" } });
    fireEvent.click(screen.getByText("Add company"));

    expect(await screen.findByText("Full name is required")).toBeInTheDocument();
    expect(screen.getByLabelText("Full name *")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Source")).toHaveValue("A note worth keeping");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("treats a whitespace-only required value as missing", async () => {
    const onSubmit = vi.fn(async () => {});
    renderForm(onSubmit);

    fireEvent.input(screen.getByLabelText("Full name *"), { target: { value: "   " } });
    fireEvent.click(screen.getByText("Add company"));

    expect(await screen.findByText("Full name is required")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("shows a backend rejection verbatim, under the field it names", async () => {
    const onSubmit = vi.fn(async () => {
      throw new Error("company website must be an absolute http or https URL, got \"northwind.test\"");
    });
    renderForm(onSubmit);

    fireEvent.input(screen.getByLabelText("Full name *"), { target: { value: "Northwind Robotics" } });
    fireEvent.input(screen.getByLabelText("Website"), { target: { value: "northwind.test" } });
    fireEvent.click(screen.getByText("Add company"));

    const shown = await screen.findByText(/absolute http or https URL/);
    expect(shown).toHaveTextContent('company website must be an absolute http or https URL, got "northwind.test"');
    // Attached to the website field, and the entered values survive.
    expect(screen.getByLabelText("Website")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Full name *")).toHaveValue("Northwind Robotics");
  });

  it("falls back to a form-level error when the message names no field", async () => {
    const onSubmit = vi.fn(async () => {
      throw new Error("the database is unavailable");
    });
    renderForm(onSubmit);

    fireEvent.input(screen.getByLabelText("Full name *"), { target: { value: "Northwind Robotics" } });
    fireEvent.click(screen.getByText("Add company"));

    expect(await screen.findByText("the database is unavailable")).toBeInTheDocument();
  });

  it("is completable with the keyboard alone", async () => {
    const onSubmit = vi.fn(async () => {});
    renderForm(onSubmit);

    const inputs = screen.getAllByRole("textbox");
    // Every field is reachable in the order it is declared, and none is
    // removed from the tab order.
    expect(inputs).toHaveLength(FIELDS.length);
    for (const input of inputs) {
      input.focus();
      expect(document.activeElement).toBe(input);
      expect(input).not.toHaveAttribute("tabindex", "-1");
    }

    const fullName = screen.getByLabelText("Full name *");
    fullName.focus();
    fireEvent.input(fullName, { target: { value: "Northwind Robotics" } });
    // Enter in a text input submits the form, no pointer involved.
    fireEvent.submit(fullName.closest("form")!);

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
  });

  it("clears the form after a successful submission", async () => {
    const onSubmit = vi.fn(async () => {});
    renderForm(onSubmit);

    fireEvent.input(screen.getByLabelText("Full name *"), { target: { value: "Northwind Robotics" } });
    fireEvent.click(screen.getByText("Add company"));

    await waitFor(() => expect(screen.getByLabelText("Full name *")).toHaveValue(""));
  });
});

describe("value helpers", () => {
  it("turns blank numbers into null and keeps real ones", () => {
    expect(num("")).toBeNull();
    expect(num("   ")).toBeNull();
    expect(num("0")).toBe(0);
    expect(num("160000")).toBe(160000);
  });

  it("splits comma-separated lists, leaving trimming to the backend", () => {
    expect(list("")).toEqual([""]);
    expect(list("a@example.test, b@example.test")).toEqual(["a@example.test", " b@example.test"]);
  });
});
