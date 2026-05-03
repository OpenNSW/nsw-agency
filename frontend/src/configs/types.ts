import { z } from 'zod';

export const UIConfigSchema = z.object({
  branding: z.object({
    appName: z.string().min(1),
    logoUrl: z.string().optional(),
    favicon: z.string().optional(),
  }),
  theme: z.object({
    fontFamily: z.string(),
    borderRadius: z.string(),
  }).optional(),
  features: z.object({
    preConsignment: z.boolean(),
    consignmentManagement: z.boolean(),
    reportingDashboard: z.boolean(),
  }).optional(),
});

export type UIConfig = z.infer<typeof UIConfigSchema>;

export function validateConfig(parsed: unknown, instanceId: string): UIConfig {
  const result = UIConfigSchema.safeParse(parsed);
  if (!result.success) {
    throw new Error(
      "Invalid configuration for " + instanceId + ":\n" +
      result.error.issues.map(i => "- " + i.path.join('.') + ": " + i.message).join('\n')
    );
  }
  return result.data;
}
