import * as SliderPrimitive from "@radix-ui/react-slider";
import { cn } from "@/lib/utils";
import { forwardRef } from "react";

const Slider = forwardRef<
  React.ElementRef<typeof SliderPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof SliderPrimitive.Root>
>(({ className, ...props }, ref) => (
  <SliderPrimitive.Root
    ref={ref}
    className={cn(
      "relative flex w-full touch-none select-none items-center",
      className,
    )}
    {...props}
  >
    <SliderPrimitive.Track className="relative h-0.5 w-full grow overflow-hidden rounded-full bg-[rgba(255,255,255,0.12)]">
      <SliderPrimitive.Range className="absolute h-full bg-[rgba(255,255,255,0.35)]" />
    </SliderPrimitive.Track>
    <SliderPrimitive.Thumb className="block h-4 w-4 rounded-full border border-[rgba(255,255,255,0.2)] bg-white shadow-md transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) hover:scale-110 disabled:pointer-events-none disabled:opacity-50" />
  </SliderPrimitive.Root>
));
Slider.displayName = SliderPrimitive.Root.displayName;

export { Slider };
