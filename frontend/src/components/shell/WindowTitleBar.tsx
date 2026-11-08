import { System } from "@wailsio/runtime";

/** macOS draws the traffic lights over the webview, so the app reserves a
    dedicated strip at the top of the window for them. Without it the lights
    float over the sidebar wordmark, and the status strip's buttons sit inside
    the native invisible-titlebar drag zone. Other platforms keep their native
    title bar, so this renders nothing there. */
export function WindowTitleBar() {
  if (!System.IsMac()) return null;

  return (
    <div
      class="window-titlebar h-10 shrink-0 border-b border-line bg-sidebar"
      aria-hidden="true"
    />
  );
}
