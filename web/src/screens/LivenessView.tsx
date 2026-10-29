import { Amplify } from "aws-amplify";
import { ThemeProvider, type Theme } from "@aws-amplify/ui-react";
import { FaceLivenessDetector, type ErrorState } from "@aws-amplify/ui-react-liveness";
import "@aws-amplify/ui-react/styles.css";
import { config } from "../config";

export interface LivenessError {
  state: ErrorState;
  error: Error;
}

// Guest credentials can only call rekognition:StartFaceLivenessSession.
Amplify.configure({
  Auth: {
    Cognito: {
      identityPoolId: config.identityPoolId,
      allowGuestAccess: true,
    },
  },
});

// Fonts, colours and radii only; the oval and light sequence are part of the check.
const theme: Theme = {
  name: "face-verify",
  tokens: {
    fonts: {
      default: {
        variable: { value: "var(--font-body)" },
        static: { value: "var(--font-body)" },
      },
    },
    colors: {
      font: {
        primary: { value: "var(--ink)" },
        secondary: { value: "var(--slate)" },
      },
      brand: {
        primary: {
          10: { value: "var(--persimmon-tint)" },
          20: { value: "var(--persimmon-tint)" },
          40: { value: "var(--persimmon)" },
          60: { value: "var(--persimmon)" },
          80: { value: "var(--persimmon)" },
          90: { value: "var(--persimmon-hover)" },
          100: { value: "var(--persimmon-active)" },
        },
      },
    },
    radii: {
      small: { value: "var(--radius)" },
      medium: { value: "var(--radius)" },
      large: { value: "var(--radius)" },
    },
  },
};

interface Props {
  sessionId: string;
  onComplete: () => Promise<void>;
  onError: (e: LivenessError) => void;
  onCancel: () => void;
}

export default function LivenessView({ sessionId, onComplete, onError, onCancel }: Props) {
  return (
    <ThemeProvider theme={theme}>
      <FaceLivenessDetector
        sessionId={sessionId}
        region={config.awsRegion}
        disableStartScreen
        // Only means streaming finished; the server fetches the actual result.
        onAnalysisComplete={onComplete}
        onError={onError}
        onUserCancel={onCancel}
      />
    </ThemeProvider>
  );
}
