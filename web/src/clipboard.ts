// navigator.clipboard requires a secure context (HTTPS, or localhost) — this
// dashboard is commonly reached over plain HTTP on a LAN/internal IP, where
// the API is simply absent or silently rejects. Fall back to the older
// execCommand("copy") path (works over HTTP, deprecated but still supported)
// so copying doesn't just fail quietly either way.
export async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the execCommand fallback below
    }
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}
