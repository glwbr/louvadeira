import { createFormHook, createFormHookContexts } from "@tanstack/react-form"
import { Loader2 } from "lucide-react"
import { useId, type ComponentProps, type ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"

const { fieldContext, formContext, useFieldContext, useFormContext } =
  createFormHookContexts()

type FieldShellProps = {
  label: ReactNode
  htmlFor?: string
  hint?: ReactNode
  error?: string
  className?: string
  children: ReactNode
}

/**
 * NOTE: Label, control, hint and error stacked the same way in every form. The error replaces the hint so the field
 * does not grow by two lines at once.
 */
export function FieldShell({
  label,
  htmlFor,
  hint,
  error,
  className,
  children,
}: FieldShellProps) {
  return (
    <div className={cn("grid min-w-0 gap-2", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : (
        hint && (
          <p className="text-xs text-pretty text-muted-foreground">{hint}</p>
        )
      )}
    </div>
  )
}

function useFieldError(): string | undefined {
  const field = useFieldContext<unknown>()
  const { isTouched, errors } = field.state.meta
  const first = errors.find(
    (error): error is string => typeof error === "string" && error.length > 0
  )
  return isTouched ? first : undefined
}

type TextFieldProps = {
  label: ReactNode
  hint?: ReactNode
  className?: string
  /** NOTE: Rewrites what was typed before it is stored, for masks like the phone number. */
  format?: (value: string) => string
} & Omit<
  ComponentProps<typeof Input>,
  "id" | "name" | "value" | "onChange" | "onBlur" | "className"
>

function TextField({
  label,
  hint,
  className,
  format,
  ...input
}: TextFieldProps) {
  const field = useFieldContext<string>()
  const error = useFieldError()
  const id = useId()
  return (
    <FieldShell
      label={label}
      htmlFor={id}
      hint={hint}
      error={error}
      className={className}
    >
      <Input
        id={id}
        name={field.name}
        value={field.state.value}
        onBlur={field.handleBlur}
        onChange={(event) =>
          field.handleChange(
            format ? format(event.target.value) : event.target.value
          )
        }
        aria-invalid={Boolean(error)}
        {...input}
      />
    </FieldShell>
  )
}

export type Option = { value: string; label: string }

type SelectFieldProps = {
  label: ReactNode
  options: readonly Option[]
  placeholder?: string
  hint?: ReactNode
  disabled?: boolean
  className?: string
  onValueChange?: (value: string) => void
}

function SelectField({
  label,
  options,
  placeholder,
  hint,
  disabled,
  className,
  onValueChange,
}: SelectFieldProps) {
  const field = useFieldContext<string>()
  const error = useFieldError()
  const id = useId()
  return (
    <FieldShell
      label={label}
      htmlFor={id}
      hint={hint}
      error={error}
      className={className}
    >
      <Select
        name={field.name}
        value={field.state.value}
        disabled={disabled}
        onValueChange={(value) => {
          field.handleChange(value)
          field.handleBlur()
          onValueChange?.(value)
        }}
      >
        <SelectTrigger id={id} className="w-full" aria-invalid={Boolean(error)}>
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FieldShell>
  )
}

function SwitchField({
  label,
  description,
}: {
  label: ReactNode
  description?: ReactNode
}) {
  const field = useFieldContext<boolean>()
  const id = useId()
  return (
    <label
      htmlFor={id}
      className="flex cursor-pointer items-center justify-between gap-4 rounded-xl border p-3.5 transition-colors hover:bg-accent/40"
    >
      <span className="min-w-0">
        <span className="block text-sm font-medium">{label}</span>
        {description && (
          <span className="mt-0.5 block text-xs text-pretty text-muted-foreground">
            {description}
          </span>
        )}
      </span>
      <Switch
        id={id}
        checked={field.state.value}
        onCheckedChange={(checked) => field.handleChange(checked)}
      />
    </label>
  )
}

/**
 * NOTE: The submit button follows the form: disabled while invalid or sending, with a spinner while sending.
 */
function SubmitButton({
  children,
  pendingLabel,
  className,
}: {
  children: ReactNode
  pendingLabel: string
  className?: string
}) {
  const form = useFormContext()
  return (
    <form.Subscribe
      selector={(state) => [state.canSubmit, state.isSubmitting] as const}
    >
      {([canSubmit, isSubmitting]) => (
        <Button
          type="submit"
          disabled={!canSubmit || isSubmitting}
          aria-busy={isSubmitting}
          className={className}
        >
          {isSubmitting && <Loader2 className="animate-spin" />}
          {isSubmitting ? pendingLabel : children}
        </Button>
      )}
    </form.Subscribe>
  )
}

function Form({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  const form = useFormContext()
  return (
    <form
      noValidate
      className={cn("flex flex-col gap-5", className)}
      onSubmit={(event) => {
        event.preventDefault()
        event.stopPropagation()
        void form.handleSubmit()
      }}
    >
      {children}
    </form>
  )
}

export const { useAppForm, withForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: { TextField, SelectField, SwitchField },
  formComponents: { Form, SubmitButton },
})

export { useFieldContext, useFormContext }
