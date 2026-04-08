import type { ComponentProps, ReactNode } from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { Toggle } from "./settingsControls";

type LabelProps = { id: string; label: string; description?: ReactNode };

export function SettingsSelectRow({
  id,
  label,
  description,
  children,
  className,
}: LabelProps & { children: ReactNode; className?: string }) {
  return (
    <div className={cn(className)}>
      <label className="mb-2 block text-sm font-medium text-(--plum-text)" htmlFor={id}>
        {label}
      </label>
      {children}
      {description != null && description !== false ? (
        <p className="mt-1.5 text-xs text-(--plum-muted)">{description}</p>
      ) : null}
    </div>
  );
}

export function SettingsInputRow({
  id,
  label,
  description,
  inputProps,
}: LabelProps & {
  inputProps: ComponentProps<typeof Input>;
}) {
  return (
    <div>
      <label className="mb-2 block text-sm font-medium text-(--plum-text)" htmlFor={id}>
        {label}
      </label>
      <Input id={id} {...inputProps} />
      {description != null ? (
        <p className="mt-2 text-xs text-(--plum-muted)">{description}</p>
      ) : null}
    </div>
  );
}

export function SettingsToggleRow({
  label,
  checked,
  onChange,
  description,
  className,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  description?: string;
  className?: string;
}) {
  return (
    <div className={cn("flex items-end", className)}>
      <Toggle label={label} checked={checked} onChange={onChange} description={description} />
    </div>
  );
}
