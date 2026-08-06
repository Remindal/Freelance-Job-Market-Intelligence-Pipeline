import { InputHTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/utils";

export const Input = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    className={cn(
      "h-9 rounded-md border border-neutral-300 bg-white px-3 text-sm outline-none",
      "focus:border-primary focus:ring-1 focus:ring-primary/30",
      "placeholder:text-neutral-400",
      className,
    )}
    {...props}
  />
));
Input.displayName = "Input";
