import React from "react";
import "./app.css";
import "./components/FlowDiagram/flow.css";
import App from "./App";
import { createRoot } from "react-dom/client";

const container = document.getElementById("root");
const root = createRoot(container!);
root.render(
  // <React.StrictMode>
  <App />
  // </React.StrictMode>
);
