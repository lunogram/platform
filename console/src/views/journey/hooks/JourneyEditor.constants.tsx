import type { ReactNode } from "react";
import {
  ActionStepIcon,
  CheckCircleIcon,
  CloseIcon,
  DelayStepIcon,
  EntranceStepIcon,
  ForbiddenIcon,
} from "@/components/icons";

export const DATA_FORMAT = "application/lunogram-journey-step";
export const STEP_STYLE = "smoothstep";

import "../JourneyEditor.css";

export const statIcons: Record<string, ReactNode> = {
  action: <ActionStepIcon />,
  delay: <DelayStepIcon />,
  completed: <CheckCircleIcon />,
  error: <ForbiddenIcon />,
  entrance: <EntranceStepIcon />,
  ended: <CloseIcon />,
};

export const stepCategoryColors = {
  entrance: "red",
  action: "blue",
  flow: "green",
  delay: "yellow",
  exit: "red",
  info: "purple",
};
