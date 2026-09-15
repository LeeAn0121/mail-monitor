import React from "react";
import ReactDOM from "react-dom/client";
import CssBaseline from "@mui/material/CssBaseline";
import { ThemeProvider } from "@mui/material/styles";
// Variable font, single ~2MB woff2 file covering every weight — far lighter
// to embed into the Go binary than the static per-weight (or subset) builds,
// which multiply into dozens/hundreds of files.
import "pretendard/dist/web/variable/PretendardVariable.css";
import theme from "./theme";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <App />
    </ThemeProvider>
  </React.StrictMode>,
);
