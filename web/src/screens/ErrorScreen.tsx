import { retryLabel, type FlowError } from "../copy";
import { Screen } from "../components/Screen";
import styles from "./ErrorScreen.module.css";

interface ErrorScreenProps {
  error: FlowError;
  onRetry: () => void;
  onRestart: () => void;
}

export function ErrorScreen({ error, onRetry, onRestart }: ErrorScreenProps) {
  return (
    <Screen title={error.title} headingClassName={styles.title}>
      <p>{error.body}</p>
      <div className="actions">
        <button type="button" className="button button-primary" onClick={onRetry}>
          {retryLabel[error.retry]}
        </button>
        {error.retry !== "restart" && (
          <button type="button" className="button button-secondary" onClick={onRestart}>
            Start again
          </button>
        )}
      </div>
    </Screen>
  );
}
