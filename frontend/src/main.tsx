import { render } from "solid-js/web";
import App from "./App";

// ponytail: the platform only decides which side of the title bar the window
// controls take, so read it straight off the runtime rather than importing it.
document.documentElement.dataset.os = (window as { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS ?? "";

render(() => <App />, document.getElementById("root")!);
