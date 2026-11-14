import { useTranslation } from "react-i18next"

import { withForm, type Option } from "@/components/form"
import type { Reason, Repeat, UnavailabilityInput } from "@/lib/api"
import { absenceLabel, REASONS, reasonLabel, WEEKDAY_INDEXES, weekdayName } from "@/lib/format"
import { t } from "@/lib/i18n"

export type AbsenceValues = {
  /** NOTE: Only the leader's form asks who; the member's form leaves it empty. */
  personId: string
  repeat: Repeat
  startDate: string
  endDate: string
  weekday: string
  weekOfMonth: string
  reason: Reason | ""
}

export function absenceDefaults(date: string): AbsenceValues {
  return { personId: "", repeat: "none", startDate: date, endDate: date, weekday: "", weekOfMonth: "", reason: "" }
}

type FieldErrors = Partial<Record<keyof AbsenceValues, string>>

export function validateAbsence(value: AbsenceValues, askPerson: boolean): { fields: FieldErrors } | undefined {
  const fields: FieldErrors = {}
  if (askPerson && !value.personId) {
    fields.personId = t("absence.errors.person")
  }
  if (!value.startDate) {
    fields.startDate = t("absence.errors.date")
  }
  if (value.repeat === "none" && !value.endDate) {
    fields.endDate = t("absence.errors.date")
  }
  if (value.endDate && value.startDate && value.endDate < value.startDate) {
    fields.endDate = t("absence.errors.endBeforeStart")
  }
  if (value.repeat !== "none" && value.weekday === "") {
    fields.weekday = t("absence.errors.weekday")
  }
  if (value.repeat === "monthly" && value.weekOfMonth === "") {
    fields.weekOfMonth = t("absence.errors.weekOfMonth")
  }
  if (!value.reason) {
    fields.reason = t("absence.errors.reason")
  }
  return Object.keys(fields).length > 0 ? { fields } : undefined
}

export function absenceInput(value: AbsenceValues): Omit<UnavailabilityInput, "personId"> {
  const repeating = value.repeat !== "none"
  return {
    repeat: value.repeat,
    startDate: value.startDate,
    endDate: value.endDate,
    reason: value.reason as Reason,
    weekday: repeating ? Number(value.weekday) : null,
    weekOfMonth: value.repeat === "monthly" ? Number(value.weekOfMonth) : null,
  }
}

/**
 * NOTE: When, how often and why someone cannot serve. Shared by the leader's and the member's forms, which differ only
 * in whether they ask who.
 */
export const AbsenceFields = withForm({
  defaultValues: absenceDefaults(""),
  render: function Render({ form }) {
    const { t } = useTranslation()
    const repeatOptions: Option[] = (["none", "weekly", "monthly"] as const).map((repeat) => ({
      value: repeat,
      label: t(`absence.repeatOption.${repeat}`),
    }))
    const weekdayOptions: Option[] = WEEKDAY_INDEXES.map((index) => ({ value: String(index), label: weekdayName(index) }))
    const weekOptions: Option[] = [1, 2, 3, 4].map((week) => ({
      value: String(week),
      label: t("absence.weekOfMonthOption", { week }),
    }))
    const reasonOptions: Option[] = REASONS.map((reason) => ({ value: reason, label: reasonLabel(reason) }))

    return (
      <>
        <form.AppField
          name="repeat"
          listeners={{
            onChange: ({ value }) =>
              form.setFieldValue("endDate", value === "none" ? form.getFieldValue("startDate") : ""),
          }}
        >
          {(field) => <field.SelectField label={t("absence.repeat")} options={repeatOptions} />}
        </form.AppField>

        <form.Subscribe selector={(state) => state.values.repeat}>
          {(repeat) =>
            repeat !== "none" && (
              <div className={repeat === "monthly" ? "grid grid-cols-2 gap-3" : "grid"}>
                {repeat === "monthly" && (
                  <form.AppField name="weekOfMonth">
                    {(field) => (
                      <field.SelectField label={t("absence.weekOfMonth")} placeholder={t("absence.choose")} options={weekOptions} />
                    )}
                  </form.AppField>
                )}
                <form.AppField name="weekday">
                  {(field) => (
                    <field.SelectField
                      label={t("absence.weekday")}
                      placeholder={t("absence.weekdayPlaceholder")}
                      options={weekdayOptions}
                    />
                  )}
                </form.AppField>
              </div>
            )
          }
        </form.Subscribe>

        <form.Subscribe selector={(state) => [state.values.repeat, state.values.startDate] as const}>
          {([repeat, startDate]) => (
            <div className="grid grid-cols-2 gap-3">
              <form.AppField
                name="startDate"
                listeners={{
                  onChange: ({ value }) => {
                    const end = form.getFieldValue("endDate")
                    if (end && value && end < value) {
                      form.setFieldValue("endDate", value)
                    }
                  },
                }}
              >
                {(field) => <field.TextField type="date" label={repeat === "none" ? t("absence.from") : t("absence.startingOn")} />}
              </form.AppField>
              <form.AppField name="endDate">
                {(field) => (
                  <field.TextField
                    type="date"
                    min={startDate}
                    label={repeat === "none" ? t("absence.to") : t("absence.untilOptional")}
                  />
                )}
              </form.AppField>
            </div>
          )}
        </form.Subscribe>

        <form.Subscribe selector={(state) => state.values}>
          {(values) =>
            values.repeat !== "none" &&
            values.weekday !== "" &&
            (values.repeat === "weekly" || values.weekOfMonth !== "") && (
              <p className="-mt-2 text-sm text-pretty text-brand">
                {values.endDate
                  ? absenceLabel(absenceInput(values))
                  : t("absence.noEnd", { label: absenceLabel(absenceInput(values)) })}
              </p>
            )
          }
        </form.Subscribe>

        <form.AppField name="reason">
          {(field) => (
            <field.SelectField
              label={t("absence.reason")}
              placeholder={t("absence.reasonPlaceholder")}
              hint={t("absence.reasonHint")}
              options={reasonOptions}
            />
          )}
        </form.AppField>
      </>
    )
  },
})
