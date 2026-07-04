// TypeScript port of github.com/veil-proto/veil/link's link.go.
//
// Format: veil://<base64url-nopad(config-text)>[#<url-escaped-name>]
//
// Pure string/encoding logic (base64url + URL component escaping), no crypto
// or config parsing involved, so this is reimplemented directly rather than
// round-tripping to the sidecar the way parseConfig/serializeConfig do.

export const SCHEME = "veil://";

export function encode(configText: string, name: string): string {
  const body = base64UrlEncode(configText);
  if (!name) return `${SCHEME}${body}`;
  return `${SCHEME}${body}#${encodeURIComponent(name)}`;
}

export function decode(link: string): { configText: string; name: string } {
  const trimmed = link.trim();
  if (!trimmed.startsWith(SCHEME)) {
    throw new Error("not a veil:// link");
  }
  const body = trimmed.slice(SCHEME.length);
  const hashIndex = body.indexOf("#");
  const encodedConfig = hashIndex >= 0 ? body.slice(0, hashIndex) : body;
  let name = "";
  if (hashIndex >= 0) {
    try {
      name = decodeURIComponent(body.slice(hashIndex + 1));
    } catch {
      throw new Error("invalid name in link");
    }
  }
  if (!/^[A-Za-z0-9_-]*$/.test(encodedConfig)) {
    throw new Error("invalid link encoding");
  }
  let configText: string;
  try {
    configText = base64UrlDecode(encodedConfig);
  } catch {
    throw new Error("invalid link encoding");
  }
  if (configText.length === 0) {
    throw new Error("empty link");
  }
  return { configText, name };
}

function base64UrlEncode(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64UrlDecode(encoded: string): string {
  let base64 = encoded.replace(/-/g, "+").replace(/_/g, "/");
  while (base64.length % 4 !== 0) base64 += "=";
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}
