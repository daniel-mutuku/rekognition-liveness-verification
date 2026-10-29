import { Screen } from "../components/Screen";

export function Welcome({ onStart }: { onStart: () => void }) {
  return (
    <Screen title="Show it's really you">
      <p>
        Upload a photo of yourself, then do a 10-second check with your camera. We compare the two and tell you
        the result straight away.
      </p>

      <div className="stack">
        <h2>What you need</h2>
        <ul>
          <li>A clear photo of just you</li>
          <li>A camera and a well-lit room</li>
        </ul>
      </div>

      <div className="actions">
        <button type="button" className="button button-primary" onClick={onStart}>
          Get verified
        </button>
      </div>

      
    </Screen>
  );
}
