import { useContext, useState } from "react";
import { Controller, useForm, type UseFormReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { Campaign, Template, User } from "@/types";
import { useTranslation } from "react-i18next";
import { ProjectContext, TemplateContext } from "@/contexts";
import * as z from "zod";

import { Input } from "@/components/ui/input";

import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";

import { UserSelection } from "../UserSelection";

const emailSetupFormSchema = z.object({
  subject: z.string("Subject is required").min(1, "Subject is required"),
  from: z.object({
    name: z.string().optional(),
    email: z.email("Invalid from email address").optional(),
  }),
  replyTo: z.email("Invalid reply-to email address").optional(),
});

const randomSubjects = [
  "You won't believe what we have for you...",
  "Your exclusive invitation inside 🎉",
  "Last chance: Don't miss out!",
  "We thought you'd love this",
  "Something special just for you",
  "Your weekly update is here",
  "Quick question for you...",
  "This might interest you",
  "A gift from us to you 🎁",
  "Breaking news you need to see",
];

function randomSubject() {
  const index = Math.floor(Math.random() * randomSubjects.length);
  return randomSubjects[index];
}

export function EmailForm(campaign: Campaign, template?: Template) {
  const formSchema = emailSetupFormSchema.extend({
    from: z.object({
      email: campaign?.provider?.data.default_from
        ? z.string().optional()
        : z.email("Invalid from email address"),
      name: campaign?.provider?.data.default_from_name
        ? z.string().optional()
        : z.string("From name is required").min(1),
    }),
  });

  const form = useForm({
    resolver: zodResolver(formSchema),
    defaultValues: {
      from: {
        name: template?.data.from?.name ?? "",
        email: template?.data.from?.email ?? "",
      },
      subject: template?.data.subject ?? randomSubject(),
      replyTo: template?.data.replyTo ?? "",
    },
  });

  return form;
}

interface EmailFormControlProps {
  campaign: Campaign;
  form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>;
  disabled?: boolean;
}

export function EmailFormControl({
  campaign,
  form,
  disabled = false,
}: EmailFormControlProps) {
  const { t } = useTranslation();

  return (
    <FieldGroup className="mt-7">
      <Controller
        name="subject"
        control={form.control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid} className="gap-2">
            <FieldLabel htmlFor="form-rhf-demo-subject">
              {t("campaign.setup.channels.email.subject.label")}
            </FieldLabel>
            <Input
              {...field}
              id="form-rhf-demo-subject"
              aria-invalid={fieldState.invalid}
              placeholder=""
              autoComplete="off"
              disabled={disabled}
              readOnly={disabled}
            />
            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
      <Controller
        name="from.name"
        control={form.control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="form-rhf-demo-fromName">
              {t("campaign.setup.channels.email.from.name.label")}
            </FieldLabel>
            <Input
              {...field}
              id="form-rhf-demo-fromName"
              aria-invalid={fieldState.invalid}
              placeholder={campaign?.provider?.data.default_from_name || ""}
              autoComplete="off"
              disabled={disabled}
              readOnly={disabled}
            />
            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
      <Controller
        name="from.email"
        control={form.control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="form-rhf-demo-fromEmail">
              {t("campaign.setup.channels.email.from.email.label")}
            </FieldLabel>
            <Input
              {...field}
              id="form-rhf-demo-fromEmail"
              aria-invalid={fieldState.invalid}
              placeholder={campaign?.provider?.data.default_from || ""}
              disabled={
                disabled || campaign?.provider?.data.default_from_locked
              }
              readOnly={disabled}
              autoComplete="off"
            />
            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
      <Controller
        name="replyTo"
        control={form.control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="form-rhf-demo-replyTo">
              {t("campaign.setup.channels.email.reply_to.label")}
            </FieldLabel>
            <Input
              {...field}
              id="form-rhf-demo-replyTo"
              aria-invalid={fieldState.invalid}
              placeholder=""
              autoComplete="off"
              disabled={disabled}
              readOnly={disabled}
            />
            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
    </FieldGroup>
  );
}

export interface EmailSetupProps {
  campaign: Campaign;
  form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>;
}

export function EmailPreview({ campaign, form }: EmailSetupProps) {
  const [project] = useContext(ProjectContext);
  const [template] = useContext(TemplateContext);
  const { t } = useTranslation();
  const [selectedUser, setSelectedUser] = useState<User | null>(null);

  const loadingSubjects = 3;
  const { subject, from } = form.watch();

  const displayFromName =
    from?.name ||
    template?.data?.from?.name ||
    campaign?.provider?.data?.default_from_name ||
    "";

  const displayFromEmail =
    from?.email ||
    template?.data?.from?.email ||
    campaign?.provider?.data?.default_from ||
    "";

  return (
    <>
      <div className="mb-4">
        <UserSelection
          projectId={project?.id}
          value={selectedUser}
          onChange={setSelectedUser}
        />
      </div>

      <div className="bg-white border rounded-md shadow-sm w-full overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-2 border-b">
          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-gray-300"
            />
            <div className="flex items-center gap-2 text-gray-400">
              <StarIcon />
              <ChevronIcon />
            </div>
          </div>
          <div className="w-2/5 flex items-center gap-1 truncate min-w-0">
            {displayFromName && (
              <span className="font-semibold text-gray-900 whitespace-nowrap">
                {displayFromName}
              </span>
            )}
            {displayFromEmail && (
              <span className="text-gray-400 whitespace-nowrap truncate">
                &lt;{displayFromEmail}&gt;
              </span>
            )}
          </div>
          <div className="w-3/5 font-semibold text-sm truncate">{subject}</div>
        </div>

        <div>
          {Array.from({ length: loadingSubjects }).map((_, index) => (
            <div key={index} className="flex items-center gap-3 px-4 py-2">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  className="h-4 w-4 rounded border-gray-300"
                  disabled
                />
                <div className="flex items-center gap-2 text-gray-400">
                  <StarIcon />
                  <ChevronIcon />
                </div>
              </div>
              <div className="w-2/5">
                <div className="h-3 w-1/2 bg-gray-200 rounded"></div>
              </div>
              <div className="w-3/5 flex gap-3">
                <div className="h-3 w-1/5 bg-gray-200 rounded"></div>
                <div className="h-3 w-4/5 bg-gray-200 rounded"></div>
              </div>
            </div>
          ))}
        </div>
      </div>
      <p className="text-center leading-7 mt-4 text-muted-foreground">
        {t("campaign.setup.channels.email.preview_disclaimer")}
      </p>
    </>
  );
}

function StarIcon() {
  return (
    <svg
      className="h-4 w-4 opacity-40"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      viewBox="0 0 24 24"
    >
      <path d="M12 17.75L18.25 21l-1.5-7 5.25-4.75-7-.75L12 2 9 8.5l-7 .75L7.25 14l-1.5 7L12 17.75z" />
    </svg>
  );
}

function ChevronIcon() {
  return (
    <svg
      className="h-4 w-4 opacity-40"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      viewBox="0 0 24 24"
    >
      <path d="M9 5l7 7-7 7" />
    </svg>
  );
}
