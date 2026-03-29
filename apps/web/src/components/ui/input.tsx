import { forwardRef } from "react";
import { cn } from "@/lib/utils";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      className={cn(
        "flex h-10 w-full rounded-[var(--radius-md)] border border-[var(--nebula-border)] bg-[var(--nebula-panel-alt)]/88 px-3 py-2 text-sm text-[var(--nebula-text)] shadow-[inset_0_1px_0_rgba(255,255,255,0.02)] placeholder:text-[var(--nebula-muted)] transition-[border-color,background-color,box-shadow] file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:border-[color-mix(in_srgb,var(--nebula-accent)_22%,var(--nebula-border))] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--nebula-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--nebula-bg)] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      ref={ref}
      {...props}
    />
  );
});
Input.displayName = "Input";

export { Input };
