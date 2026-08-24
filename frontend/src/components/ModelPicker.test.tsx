import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@solidjs/testing-library";
import ModelPicker, { gb } from "./ModelPicker";
import type { PickerOption } from "./ModelPicker";

// Pure component: no bindings, everything arrives as props.
const options: PickerOption[] = [
  { role: "generate", model: "small-model", purpose: "Writes things", power: "fastest", approxBytes: 2 * 1024 ** 3, installed: true },
  { role: "generate", model: "big-model", purpose: "Writes things", power: "most capable", approxBytes: 6 * 1024 ** 3, installed: false },
];

const renderPicker = (over: Partial<Parameters<typeof ModelPicker>[0]> = {}) => {
  const onAssign = vi.fn();
  render(() => (
    <ModelPicker role="generate" options={options} current="" freeDiskBytes={10 * 1024 ** 3} busy={false} onAssign={onAssign} {...over} />
  ));
  return onAssign;
};

describe("ModelPicker", () => {
  it("describes each model with its power and size, marking installed ones", () => {
    renderPicker();
    const select = screen.getByLabelText("Model for generate") as HTMLSelectElement;
    const labels = Array.from(select.options).map((o) => o.text);
    expect(labels.some((l) => l.includes("small-model") && l.includes("fastest") && l.includes("2.0 GB") && l.includes("installed"))).toBe(true);
    expect(labels.some((l) => l.includes("big-model") && l.includes("most capable") && !l.includes("installed"))).toBe(true);
  });

  it("assigns the chosen model", () => {
    const onAssign = renderPicker();
    fireEvent.change(screen.getByLabelText("Model for generate"), { target: { value: "big-model" } });
    fireEvent.click(screen.getByLabelText("Assign a model to generate"));
    expect(onAssign).toHaveBeenCalledWith("big-model");
  });

  it("reveals a text input for a custom model and assigns the typed name", () => {
    const onAssign = renderPicker();
    fireEvent.change(screen.getByLabelText("Model for generate"), { target: { value: "__custom__" } });
    fireEvent.input(screen.getByLabelText("Custom model for generate"), { target: { value: "mystery:7b" } });
    fireEvent.click(screen.getByLabelText("Assign a model to generate"));
    expect(onAssign).toHaveBeenCalledWith("mystery:7b");
  });

  it("is disabled while a download is in progress", () => {
    renderPicker({ busy: true });
    expect(screen.getByLabelText("Model for generate")).toBeDisabled();
    expect(screen.getByLabelText("Assign a model to generate")).toBeDisabled();
  });

  it("refuses a model bigger than the free disk space, and says why", () => {
    const onAssign = renderPicker({ freeDiskBytes: 3 * 1024 ** 3 });
    fireEvent.change(screen.getByLabelText("Model for generate"), { target: { value: "big-model" } });
    expect(screen.getByLabelText("Assign a model to generate")).toBeDisabled();
    expect(screen.getByText(/needs 6\.0 GB.*3\.0 GB free/)).toBeTruthy();
    expect(onAssign).not.toHaveBeenCalled();
  });

  it("does not refuse an already-installed model on a full disk", () => {
    renderPicker({ freeDiskBytes: 1 * 1024 ** 3 });
    fireEvent.change(screen.getByLabelText("Model for generate"), { target: { value: "small-model" } });
    expect(screen.getByLabelText("Assign a model to generate")).not.toBeDisabled();
  });
});

describe("gb", () => {
  it("formats bytes as gigabytes with one decimal", () => {
    expect(gb(2 * 1024 ** 3)).toBe("2.0 GB");
    expect(gb(0)).toBe("0.0 GB");
  });
});
