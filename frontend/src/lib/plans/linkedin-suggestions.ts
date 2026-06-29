import {
  buildDefaultLinkedInInputValues,
  materializeLinkedInInputValues,
  type LinkedInMaterialization,
} from "$lib/plans/linkedin-template-inputs";

export interface LinkedInSuggestionInput {
  topic: string;
  installationIdsByStep: Record<string, string>;
  today: Date;
}

export type LinkedInSuggestion = LinkedInMaterialization;

export function buildLinkedInSuggestion(
  input: LinkedInSuggestionInput,
): LinkedInSuggestion {
  const values = buildDefaultLinkedInInputValues(input.today);
  values.theme = input.topic.trim() || values.theme;
  return materializeLinkedInInputValues(values, input.installationIdsByStep);
}
