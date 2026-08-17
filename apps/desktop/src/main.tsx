import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Toaster } from "sonner";

import App from "./App";
import { TooltipProvider } from "./components/ui/tooltip";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <TooltipProvider delayDuration={350}>
      <App />
      <Toaster
        position="bottom-right"
        toastOptions={{
          classNames: {
            toast: "realbrowser-toast",
          },
        }}
      />
    </TooltipProvider>
  </StrictMode>,
);
