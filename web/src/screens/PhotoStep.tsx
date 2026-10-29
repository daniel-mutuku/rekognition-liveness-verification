import { useRef, useState, type ChangeEvent, type DragEvent } from "react";
import type { Reason } from "../api";
import { photoReasonText } from "../copy";
import type { Photo } from "../flow";
import { PhotoFrame } from "../components/PhotoFrame";
import { Screen } from "../components/Screen";
import styles from "./PhotoStep.module.css";

interface PhotoStepProps {
  photo?: Photo;
  rejected?: Reason[];
  preparing: boolean;
  uploading: boolean;
  onFile: (file: File) => void;
  onUpload: () => void;
}

// capture="user" skips the gallery on most phones, so the selfie gets its own input.
const isTouch = typeof window !== "undefined" && window.matchMedia("(pointer: coarse)").matches;

export function PhotoStep({ photo, rejected, preparing, uploading, onFile, onUpload }: PhotoStepProps) {
  const gallery = useRef<HTMLInputElement>(null);
  const selfie = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const busy = preparing || uploading;

  function picked(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ""; // re-picking the same file must still fire change
    if (file) onFile(file);
  }

  function dropped(e: DragEvent) {
    e.preventDefault();
    setDragging(false);
    const file = e.dataTransfer.files[0];
    if (file && !busy) onFile(file);
  }

  const choose = () => gallery.current?.click();
  const chooseLabel = photo || rejected ? "Choose another photo" : "Choose photo";

  return (
    <Screen title="Add your profile photo">
      {rejected ? (
        <div className="stack">
          <h2 className={styles.rejectedTitle}>This photo won't work</h2>
          <ul className={styles.reasons}>
            {rejected.map((code) => (
              <li key={code}>{photoReasonText(code)}</li>
            ))}
          </ul>
        </div>
      ) : (
        <p>Face the camera, good light, just you. A selfie is fine.</p>
      )}

      <div
        className={[styles.slot, dragging && styles.dragging].filter(Boolean).join(" ")}
        onDragOver={(e) => {
          e.preventDefault();
          if (!busy) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={dropped}
      >
        {photo ? (
          <PhotoFrame src={photo.url} alt="The profile photo you picked" dimmed={!!rejected} />
        ) : (
          <div className={styles.empty}>
            {preparing ? "Preparing your photo…" : isTouch ? "Your photo goes here" : "Drop a photo here, or choose one"}
          </div>
        )}
      </div>

      <div className="actions">
        {photo && !rejected ? (
          <>
            <button type="button" className="button button-primary" onClick={onUpload} disabled={busy}>
              {uploading && <span className="spinner" aria-hidden="true" />}
              {uploading ? "Checking photo…" : "Upload photo"}
            </button>
            <button type="button" className="button button-secondary" onClick={choose} disabled={busy}>
              Choose another photo
            </button>
          </>
        ) : (
          <>
            <button type="button" className="button button-primary" onClick={choose} disabled={busy}>
              {chooseLabel}
            </button>
            {isTouch && (
              <button type="button" className="button button-secondary" onClick={() => selfie.current?.click()} disabled={busy}>
                Take a selfie
              </button>
            )}
          </>
        )}
      </div>

      <input ref={gallery} type="file" accept="image/*" className="visually-hidden" tabIndex={-1} aria-label="Choose photo" onChange={picked} />
      <input ref={selfie} type="file" accept="image/*" capture="user" className="visually-hidden" tabIndex={-1} aria-label="Take a selfie" onChange={picked} />
    </Screen>
  );
}
