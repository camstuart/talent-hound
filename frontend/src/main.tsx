import { render } from "solid-js/web";
import { System } from "@wailsio/runtime";
import App from "./App";

// Which side of the title bar the window controls take is the only thing the
// platform decides here. The runtime answers asynchronously, so the stylesheet
// holds the macOS corner open until we know otherwise: that is the platform
// this ships on first, and a corner reserved a moment too long is invisible
// where a corner reserved a moment too late covers the title.
System.Environment()
  .then((env) => {
    document.documentElement.dataset.os = env.OS;
  })
  .catch(() => {
    // A plain browser has no runtime to ask. The default layout stands.
  })
  .finally(() => {
    // Render only once the platform question is settled, so no component ever
    // reads data-os before the runtime has answered it.
    render(() => <App />, document.getElementById("root")!);
  });
