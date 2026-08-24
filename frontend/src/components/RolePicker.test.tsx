import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@solidjs/testing-library";
import RolePicker, { forRole, gb } from "./RolePicker";
import type { PickerOption } from "./RolePicker";

// Pure component: no bindings, everything arrives as props.
const options: PickerOption[] = [
  { role: "generate", model: "small-model", purpose: "", power: "fastest", approxBytes: 2 * 1024 ** 3, installed: true, pulling: false },
  { role: "generate", model: "big-model", purpose: "", power: "most capable", approxBytes: 6 * 1024 ** 3, installed: false, pulling: false },
  { role: "embed", model: "embed-model", purpose: "", power: "recommended", approxBytes: 1024 ** 3, installed: true, pulling: false },
  { role: "", model: "my-custom", purpose: "", power: "", approxBytes: 0, installed: true, pulling: false },
];

const renderPicker = (current = "") => {
  const onSelect = vi.fn();
  render(() => <RolePicker role="generate" options={options} current={current} onSelect={onSelect} />);
  return onSelect;
};

describe("RolePicker", () => {
  it("offers only installed models for the role, plus role-less custom models", () => {
    renderPicker();
    const select = screen.getByLabelText("Model for generate") as HTMLSelectElement;
    const values = Array.from(select.options).map((o) => o.value);
    expect(values).toContain("small-model");
    expect(values).toContain("my-custom");
    expect(values).not.toContain("big-model");
    expect(values).not.toContain("embed-model");
  });

  it("shows the persisted assignment as the selected option", () => {
    renderPicker("small-model");
    const select = screen.getByLabelText("Model for generate") as HTMLSelectElement;
    expect(select.value).toBe("small-model");
  });

  it("keeps a current assignment selectable even when it is not in the list", () => {
    renderPicker("vanished-model");
    const select = screen.getByLabelText("Model for generate") as HTMLSelectElement;
    expect(select.value).toBe("vanished-model");
  });

  it("assigns immediately on selection, with no extra button", () => {
    const onSelect = renderPicker();
    fireEvent.change(screen.getByLabelText("Model for generate"), { target: { value: "small-model" } });
    expect(onSelect).toHaveBeenCalledWith("small-model");
    expect(screen.queryByText("Assign")).not.toBeInTheDocument();
  });
});

describe("forRole", () => {
  it("filters to installed models suitable for the role", () => {
    expect(forRole(options, "embed").map((o) => o.model)).toEqual(["embed-model", "my-custom"]);
  });
});

describe("gb", () => {
  it("formats bytes as gigabytes with one decimal", () => {
    expect(gb(2 * 1024 ** 3)).toBe("2.0 GB");
  });
});
