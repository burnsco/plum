import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { forwardRef } from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[var(--radius-md)] text-sm font-medium transition-[background-color,border-color,color,box-shadow] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--plum-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--plum-bg)] disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "border border-[var(--plum-accent)] bg-[var(--plum-accent)] text-black shadow-sm hover:bg-[color-mix(in_srgb,var(--plum-accent)_85%,white_15%)] hover:border-[color-mix(in_srgb,var(--plum-accent)_85%,white_15%)]",
        outline:
          "border border-[var(--plum-border)] bg-transparent text-[var(--plum-text)] hover:bg-[var(--plum-panel-alt)] hover:border-[rgba(255,255,255,0.14)]",
        secondary:
          "border border-[var(--plum-border)] bg-[rgba(30,30,30,0.9)] text-[var(--plum-text)] hover:border-[rgba(255,255,255,0.14)] hover:bg-[rgba(38,38,38,0.95)]",
        ghost:
          "text-[var(--plum-muted)] hover:bg-[rgba(255,255,255,0.06)] hover:text-[var(--plum-text)]",
        link: "text-[var(--plum-accent)] underline-offset-4 hover:underline",
        icon:
          "border border-transparent bg-transparent text-[var(--plum-muted)] hover:border-[var(--plum-border)] hover:bg-[rgba(255,255,255,0.06)] hover:text-[var(--plum-text)]",
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
