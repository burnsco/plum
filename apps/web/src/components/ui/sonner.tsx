import { Toaster as Sonner } from "sonner";

export function Toaster() {
  return (
    <Sonner
      position="bottom-right"
      theme="dark"
      closeButton
      toastOptions={{
        classNames: {
          toast:
            "border border-[var(--plum-border)] bg-[var(--plum-panel)] text-[var(--plum-text)] shadow-lg",
          title: "text-[var(--plum-text)]",
          description: "text-[var(--plum-muted)]",
          success: "!border-emerald-500/35",
          error: "!border-red-500/35",
        },
      }}
    />
  );
}
