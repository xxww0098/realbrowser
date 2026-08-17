import * as React from "react";

import { cn } from "../../lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-10 w-full min-w-0 rounded-[13px] border border-border-strong bg-background px-3.5 text-[13px] text-foreground shadow-[0_1px_2px_rgba(16,24,40,.02)] outline-none transition-[border-color,box-shadow] placeholder:text-placeholder focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-primary/12 disabled:cursor-not-allowed disabled:bg-muted disabled:opacity-65",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
