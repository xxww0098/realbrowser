import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";

import { cn } from "../../lib/utils";

const buttonVariants = cva(
  "inline-flex select-none items-center justify-center gap-1.5 whitespace-nowrap rounded-[13px] text-[13px] font-semibold transition-[background-color,color,border-color,box-shadow,transform] duration-150 outline-none focus-visible:ring-3 focus-visible:ring-primary/22 disabled:pointer-events-none disabled:opacity-45 active:translate-y-px [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-[0_8px_18px_-9px_rgba(40,85,255,.78)] hover:bg-primary/92",
        outline:
          "border border-border-strong bg-background text-foreground hover:border-primary/55 hover:bg-primary-soft hover:text-primary",
        ghost: "text-muted-foreground hover:bg-muted hover:text-foreground",
        subtle: "bg-primary-soft text-primary hover:bg-primary-soft/72",
        destructive: "bg-destructive-soft text-destructive hover:bg-destructive-soft/72",
      },
      size: {
        default: "h-10 px-4",
        sm: "h-8 rounded-[10px] px-3 text-xs",
        lg: "h-11 px-5 text-[13px]",
        icon: "size-9 rounded-[12px]",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean }) {
  const Component = asChild ? Slot : "button";
  return (
    <Component
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
