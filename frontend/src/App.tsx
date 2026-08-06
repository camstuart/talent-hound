import { createSignal } from "solid-js";
import { GreetService } from "../bindings/camstuart/talent-hound";

export default function App() {
  const [name, setName] = createSignal("");
  const [greeting, setGreeting] = createSignal("");

  const greet = async () => {
    setGreeting(await GreetService.Greet(name()));
  };

  return (
    <main class="container">
      <h1>Talent Hound</h1>
      <div class="greet-row">
        <input
          type="text"
          placeholder="Your name"
          value={name()}
          onInput={(e) => setName(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && greet()}
        />
        <button onClick={greet}>Greet</button>
      </div>
      {greeting() && <p class="greeting">{greeting()}</p>}
    </main>
  );
}
