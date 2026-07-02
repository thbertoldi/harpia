import { buildDefaultLinkedInInputValues } from "$lib/plans/linkedin-template-inputs";
import { parameterValuesJson } from "$lib/plans/template-inputs";

export interface LinkedInSuggestionInput {
  topic: string;
  installationIdsByStep: Record<string, string>;
  today: Date;
}

export interface LinkedInSuggestion {
  parameterValuesJson: string;
}

export function buildLinkedInSuggestion(
  input: LinkedInSuggestionInput,
): LinkedInSuggestion {
  const values = buildDefaultLinkedInInputValues(input.today);
  values.theme = input.topic.trim() || values.theme;
  values.aggregateSourceGroupInstallationId =
    input.installationIdsByStep["fetch-news"] ?? "";
  return { parameterValuesJson: parameterValuesJson(values) };
}
