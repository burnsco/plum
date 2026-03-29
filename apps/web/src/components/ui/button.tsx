import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { forwardRef } from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[var(--radius-md)] text-sm font-medium transition-[background-color,border-color,color,box-shadow] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--nebula-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--nebula-bg)] disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "border border-[var(--nebula-accent)] bg-[var(--nebula-accent)] text-slate-950 shadow-[0_10px_24px_rgba(56,189,248,0.16)] hover:bg-[#5cc8f8] hover:border-[#5cc8f8]",
        outline:
          "border border-[var(--nebula-border)] bg-[var(--nebula-panel)]/80 text-[var(--nebula-text)] hover:bg-[var(--nebula-panel-alt)] hover:border-[color-mix(in_srgb,var(--nebula-accent)_28%,var(--nebula-border))]",
        secondary:
          "border border-[var(--nebula-border)] bg-[var(--nebula-panel-alt)]/90 text-[var(--nebula-text)] hover:border-[color-mix(in_srgb,var(--nebula-accent)_18%,var(--nebula-border))] hover:bg-[color-mix(in_srgb,var(--nebula-panel-alt)_75%,white_5%)]",
        ghost:
          "text-[var(--nebula-muted)] hover:bg-[var(--nebula-panel-alt)]/70 hover:text-[var(--nebula-text)]",
        link: "text-[var(--nebula-accent)] underline-offset-4 hover:underline",
        icon:
          "border border-transparent bg-transparent text-[var(--nebula-muted)] hover:border-[var(--nebula-border)] hover:bg-[var(--nebula-panel-alt)]/70 hover:text-[var(--nebula-text)]",
      },
      size: {
        default: "h-10 px-4 py-2",
        sm: "h-8 rounded-[var(--radius-sm)] px-3 text-xs",
        lg: "h-11 rounded-[var(--radius-lg)] px-6",
        icon: "h-10 w-10",
        "icon-sm": "h-8 w-8",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
    );
  },
);
Button.displayName = "Button";

export { Button, buttonVariants };
