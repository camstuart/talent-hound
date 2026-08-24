import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@solidjs/testing-library";
import ModelLibrary from "./ModelLibrary";
import type { PickerOption } from "./RolePicker";

const GB = 1024 ** 3;
const models: PickerOption[] = [
  { role: "embed", model: "embed-model", purpose: "Embeds", power: "recommended", approxBytes: 1 * GB, installed: true, pulling: false },
  { role: "generate", model: "big-model", purpose: "Writes", power: "most capable", approxBytes: 6 * GB, installed: false, pulling: false },
  { role: "generate", model: "mid-model", purpose: "Writes", power: "recommended", approxBytes: 2 * GB, installed: false, pulling: true },
  { role: "", model: "my-custom", purpose: "", power: "", approxBytes: 0, installed: true, pulling: false },
];

const renderLibrary = (freeDiskBytes = 10 * GB) => {
  const onPull = vi.fn();
  render(() => <ModelLibrary models={models} freeDiskBytes={freeDiskBytes} onPull={onPull} />);
  return onPull;
};

describe("ModelLibrary", () => {
  it("lists downloaded and downloading models with their state", () => {
    renderLibrary();
    const library = screen.getByRole("region", { name: "Model library" });
    expect(library).toHaveTextContent(/embed-model.*installed/);
    expect(library).toHaveTextContent(/mid-model.*downloading now/);
    expect(library).toHaveTextContent(/my-custom.*installed/);
    // Not yet downloaded models wait behind Add model.
    expect(library).not.toHaveTextContent(/big-model/);
  });

  it("lists a model once even when the catalog offers it for two roles", () => {
    const onPull = vi.fn();
    const twice: PickerOption[] = [
      { role: "classify", model: "qwen2.5:7b-instruct", purpose: "Reads", power: "recommended", approxBytes: 4 * GB, installed: true, pulling: false },
      { role: "generate", model: "qwen2.5:7b-instruct", purpose: "Writes", power: "recommended", approxBytes: 4 * GB, installed: true, pulling: false },
    ];
    render(() => <ModelLibrary models={twice} freeDiskBytes={10 * GB} onPull={onPull} />);
    expect(screen.getAllByText(/qwen2\.5:7b-instruct/)).toHaveLength(1);
  });

  it("shows how much disk space is free", () => {
    renderLibrary();
    expect(screen.getByText(/10\.0 GB free on this disk/)).toBeInTheDocument();
  });

  it("offers the not-yet-downloaded models behind Add model and starts a download", () => {
    const onPull = renderLibrary();
    fireEvent.click(screen.getByLabelText("Add model"));
    fireEvent.click(screen.getByLabelText("Download big-model"));
    expect(onPull).toHaveBeenCalledWith("big-model");
  });

  it("downloads a custom model by typed name", () => {
    const onPull = renderLibrary();
    fireEvent.click(screen.getByLabelText("Add model"));
    fireEvent.input(screen.getByLabelText("Custom model name"), { target: { value: " mystery:7b " } });
    fireEvent.click(screen.getByLabelText("Download the custom model"));
    expect(onPull).toHaveBeenCalledWith("mystery:7b");
  });

  it("refuses a download bigger than the free disk space, and says why", () => {
    const onPull = renderLibrary(3 * GB);
    fireEvent.click(screen.getByLabelText("Add model"));
    expect(screen.getByLabelText("Download big-model")).toBeDisabled();
    expect(screen.getByText(/needs 6\.0 GB.*3\.0 GB free/)).toBeTruthy();
    fireEvent.click(screen.getByLabelText("Download big-model"));
    expect(onPull).not.toHaveBeenCalled();
  });
});
